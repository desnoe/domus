# homecode

Infrastructure as Code (IaC) repository for provisioning, hardening, and managing home Debian servers, encrypted storage stacks, and automated cloud backups.

---

## 📌 Features & Architecture Overview

- **Host Provisioning**: Bootstrap minimal Debian systems with automated user management (`ansible_user`), SSH hardening, and sudo rights.
- **Layered Storage Security**:
  - **LUKS**: Block-level encryption with AES-256 (`luks_pass`) on each member device.
  - **Btrfs**: Filesystem management directly on decrypted LUKS containers with native RAID1 mirroring for multi-device pools, declarative subvolume mount paths, transparent zstd compression, and automated fstab integration.
- **Automated Backups**:
  - Daily read-only Btrfs snapshots (`btrfs-snapshot-<name>.timer` / `.service` at 05:00) bound to source and destination `.mount` units and sandboxed with read-only source protection.
  - Multi-cloud synchronization via `rclone` (`rclone-sync-<slug>.timer` / `.service` starting at 04:00) using direct `/usr/bin/rclone sync` execution, kernel mount binding, and `ProtectSystem=strict` sandboxing.
- **System Hardening**: Baseline configuration covering `ufw` firewall (dual-stack IPv4/IPv6), SSH keys, locales, NTP, and administration tools.

---

## 🛠️ Prerequisites & Setup

### 1. Tooling Requirements
- **Python**: Python 3.11 (configured via `.python-version`).
- **uv**: Fast Python package and virtual environment manager (`brew install uv`).
- **sshpass**: Required if bootstrapping with SSH password (`brew install esolitos/ipa/sshpass` or `sudo apt install sshpass`).

### 2. Environment Initialization
Clone the repository and set up the local `.venv` and Ansible collections:

```bash
# Sync dependencies with uv
uv sync

# Install pinned Ansible collections locally into ansible/collections/
uv run ansible-galaxy collection install -r ansible/requirements.yml -p ansible/collections
```

### 3. Ansible Vault Password
All sensitive credentials (`luks_pass`) are encrypted using `ansible-vault`. Ensure your vault password is saved in `~/.ansible_vault_pass` with restricted permissions:

```bash
echo "YOUR_VAULT_PASSWORD" > ~/.ansible_vault_pass
chmod 600 ~/.ansible_vault_pass
```

---

## 🔑 Where Variables and Secrets are Defined

The repository uses a clear variable precedence structure:

| Variable Type | Location | Purpose | Examples |
| :--- | :--- | :--- | :--- |
| **Global Defaults** | `ansible/envs/<env>/group_vars/all/main.yml` | Common across all hosts | `ansible_user` |
| **Host Variables** | `ansible/envs/<env>/host_vars/<host>/main.yml` | Device layout & backup targets | `crypto_disks`, `snapshot_jobs`, `rclone_jobs` |
| **Encrypted Secrets** | `ansible/envs/<env>/host_vars/<host>/vault.yml` | Storage decryption passphrases | `luks_pass` |


### User Authentication & Storage Passphrases
- **Storage Passphrase (`luks_pass`)**: The decryption passphrase for LUKS block devices is encrypted in each host's **`ansible/envs/<env>/host_vars/<host>/vault.yml`** and consumed by the **`crypto`** role. An unencrypted reference template is provided in [`ansible/vault.yml.example`](ansible/vault.yml.example).
- **User Login & Sudo**: Human user passwords are set during the initial Debian OS installation and are **never stored or managed in version control**. Ansible manages authorized SSH keys (`sshd` role) and passwordless sudo privileges via `/etc/sudoers.d/99-ansible-wheel` (`sudo` role).

To view or edit secrets:
```bash
# View encrypted secrets
uv run ansible-vault view ansible/envs/lab/host_vars/debian.delarche.fr/vault.yml

# Edit secrets in place
uv run ansible-vault edit ansible/envs/lab/host_vars/debian.delarche.fr/vault.yml
```

---

## 🚀 Playbooks & Common Workflows

Always run playbooks using `uv run` from the repository root:

### 1. Bootstrap a Fresh Debian Server (Day 0)
Configures initial administrative access (SSH keys, passwordless sudo via `wheel`) and automatically chains full system baseline configuration:
```bash
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/bootstrap.yml -K
```

### 2. Configure System Baseline (OS & Security)
Applies package updates, base tools, firewall (`ufw`), time sync (`chrony`/`ntp`), user configuration, and SSH hardening without touching storage:
```bash
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/system.yml
```

### 3. Unlock Storage After Server Reboot
Quickly unlocks LUKS containers, scans multi-device Btrfs pools, and mounts subvolumes:
```bash
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/storage-unlock.yml
```

### 4. Wipe & Reset Storage (Destructive)
Unmounts, wipes LUKS containers, re-encrypts, formats fresh Btrfs multi-device RAID1, and sets up subvolumes. Includes an interactive confirmation prompt:
```bash
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-wipe-and-reset.yml
```

### 5. Replace a Failed Disk in Btrfs RAID1
Assisted workflow to format LUKS on a replacement drive, mount degraded if necessary, execute `btrfs replace start`, and backup the new LUKS header:
```bash
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-replace.yml
```

### 6. Configure Backup Services (Snapshots & Rclone)
Copies your local `~/.config/rclone/rclone.conf` (`0600`), deploys `/home/{{ ansible_user }}/bin/snapshot.sh`, and configures sandboxed `systemd.service` and `systemd.timer` units for automated Btrfs snapshotting and cloud sync:
```bash
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/backup.yml
```

### 7. Check Storage and System Health (Read-Only)
Inspects base system identity/uptime, locales/keyboard, time/NTP sync, SSH daemon/authorized keys, UFW firewall status, LUKS encryption status, header backup presence, SMART health, Btrfs usage/error stats, `btrfs-snapshot-*` / `rclone-sync-*` systemd timers, `journald` logs, and `rclone` remote configuration status:
```bash
# Full health report
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml

# Scoped report using tags (base, locales, ntp, sshd, ufw, crypto, snapshot, rclone)
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags crypto
```

---

## 📚 Documentation & Guidelines

- **[Detailed Ansible Architecture & Roles Guide](docs/ansible.md)**: Deep dive into all playbooks, roles, and storage layout.
- **[Test Book & Non-Regression Verification Guide](docs/testing.md)**: Complete catalog of QA, system, firewall, storage, backup, and chaos test cases (`TC-*`).
- **[Disaster Recovery & Storage Operations Guide](docs/disaster-recovery.md)**: Step-by-step procedures for emergency recovery, file restoration, and LUKS header repair.
- **[AI Agent & Contributor Guidelines](AGENTS.md)**: Language policy (mandatory English), coding standards, and pre-commit rules.

---

## 🛠️ Utility Scripts (`scripts/`)

The repository includes helper shell utilities for batch image processing via ImageMagick (`magick`):

- **[`scripts/image-convert.sh`](scripts/image-convert.sh)**: Parameterized CLI supporting A4 canvas centering (`--a4`), directory placement (`--alongside`), and quality adjustments.
- **[`scripts/png2jpg.sh`](scripts/png2jpg.sh)**: Converts PNG images to optimized JPEG.
- **[`scripts/png2jpga4.sh`](scripts/png2jpga4.sh)**: Centers PNG images onto a 300 DPI A4 white canvas.
- **[`scripts/scan2jpg.sh`](scripts/scan2jpg.sh)**: Converts image scans alongside original source paths.

---

## 🧪 Code Quality & Pre-commit

All contributions must pass the full pre-commit test suite before committing:

```bash
uv run pre-commit run --all-files
```
Enforces `ansible-lint`, `yamllint`, `shellcheck`, and standard security checks.
