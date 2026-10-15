# domus

Infrastructure as Code (IaC) repository for provisioning, hardening, and managing home Debian servers, encrypted storage stacks, and automated cloud backups.

---

## 📌 Features & Architecture Overview

- **Host Provisioning**: Bootstrap minimal Debian systems with automated user management (`ansible_user`), SSH hardening, and sudo rights.
- **Layered Storage Security**:
  - **LUKS**: Block-level encryption with AES-256 (`luks_pass`) on each member device, featuring inline hardware AES acceleration (`crypto_perf_flags`) bypassing asynchronous workqueues.
  - **Btrfs**: Filesystem management directly on decrypted LUKS containers with native RAID1 mirroring for multi-device pools, declarative subvolume mount paths, transparent zstd compression (with declarative per-directory uncompressed opt-outs via `crypto_no_compression_paths`), and automated fstab integration.
- **Automated Backups**:
  - Daily read-only Btrfs snapshots (`btrfs-snapshot-<name>.timer` / `.service` at 05:00) bound to source and destination `.mount` units and sandboxed with read-only source protection.
  - Multi-cloud synchronization via `rclone` (`rclone-sync-<slug>.timer` / `.service` starting at 04:00) using direct `/usr/bin/rclone sync` execution, kernel mount binding, `ProtectSystem=strict` sandboxing, and gentle background scheduling (`Nice=10`, `IOSchedulingClass=best-effort`, `IOSchedulingPriority=6`).
- **File Sharing & macOS Integration**:
  - Samba SMB3 service on encrypted storage hosts exposing Btrfs subvolumes in read-only mode with native macOS Finder "Previous Versions" snapshot navigation (`vfs_shadow_copy2`).
  - Automated Mac backups via `rsync` over SSH into target directory `/home/olivier/data/sync/mac/`.
- **Unified Observability & Health Telemetry**:
  - Grafana Alloy collector daemon deployed across all nodes via native systemd sandboxing (`ProtectSystem=strict`, `MemoryMax=512M`, `Nice=15`).
  - Streams host metrics (CPU, RAM, temperatures), Btrfs filesystem and LUKS mapper I/O statistics (`prometheus.exporter.unix`) to Grafana Cloud Mimir.
  - Ships structured, whitelist-filtered systemd journal logs (`loki.source.journal`) to Grafana Cloud Loki.
  - LAN-accessible internal web UI (`12345/tcp`) protected by UFW firewall rules.
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
| **Host Variables** | `ansible/envs/<env>/host_vars/<host>/main.yml` | Device layout & backup targets | `crypto_disks`, `snapshot_jobs`, `rclone_jobs`, `samba_shares` |
| **Encrypted Secrets** | `ansible/envs/<env>/host_vars/<host>/vault.yml` | Storage passphrases & credentials | `luks_pass`, `samba_pass`, `vault_grafana_cloud_api_key` |


### User Authentication & Storage Passphrases
- **Storage Passphrase (`luks_pass`)**: The decryption passphrase for LUKS block devices is encrypted in each host's **`ansible/envs/<env>/host_vars/<host>/vault.yml`** and consumed by the **`crypto`** role. An unencrypted reference template is provided in [`ansible/vault.yml.example`](ansible/vault.yml.example).
- **Samba User Credentials (`samba_pass`)**: Password for accessing Samba network shares over SMB3 from macOS Finder, encrypted in **`vault.yml`** and provisioned via `smbpasswd`.
- **Grafana Cloud Credentials (`vault_grafana_cloud_api_key`)**: Access policy token for streaming metrics and logs to Grafana Cloud Mimir and Loki endpoints.
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

### 2. Configure System Baseline & Services (OS, Security, Runner & Samba)
Applies package updates, base tools, firewall (`ufw`), time sync (`chrony`/`ntp`), user configuration, SSH hardening, GitHub Actions runner provisioning, and Samba file sharing (`samba` role on `crypto` hosts):
```bash
# Full system run
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/system.yml

# Scoped run targeting Samba only
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/system.yml --tags samba

# Scoped run targeting Alloy telemetry only
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/system.yml --tags alloy
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

### 6. Add a Member Disk to a Storage Pool
Formats LUKS on an additional drive, adds it to the pool, and converts from single-disk to RAID1 or performs a fast filtered allocation balance:
```bash
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-disk-add.yml -e pool=data
```

### 7. Remove a Member Disk from a RAID1 Pool
Safely converts profiles (if shrinking to 1 disk), evicts member drive via Btrfs, wipes signatures/LUKS headers, and compacts mapper indices online with zero downtime:
```bash
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-disk-remove.yml -e pool=data -e disk_index=1
```

### 8. Decommission & Remove an Encrypted Storage Pool
Permanently unmounts, closes, and wipes an encrypted storage pool (both single disk or multi-disk RAID pool). Includes interactive selection and safety confirmation (or non-interactive `-e remove_pool=<pool> -e confirm_remove=yes`):
```bash
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-remove.yml
```

### 9. Normalize Unicode Filenames (NFC/NFD)
Converts filenames on encrypted storage pools to UTF-8 NFC normalization via `convmv` to ensure compatibility between macOS Finder, Samba, and Linux:
```bash
# Default path (/home/{{ ansible_user }}/backup/sync/googledrive/macbookpro)
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/storage-normalize-unicode.yml

# Custom target path
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/storage-normalize-unicode.yml -e target_path="/home/olivier/backup/sync/googledrive/macbookpro"
```

### 10. Configure Backup Services (Snapshots & Rclone)
Copies your local `~/.config/rclone/rclone.conf` (`0600`), deploys `/home/{{ ansible_user }}/bin/snapshot.sh`, and configures sandboxed `systemd.service` and `systemd.timer` units for automated Btrfs snapshotting and cloud sync:
```bash
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/backup.yml
```

### 11. Check Storage and System Health (Read-Only)
Inspects base system identity/uptime, locales/keyboard, time/NTP sync, SSH daemon/authorized keys, UFW firewall status, LUKS encryption status, header backup presence, SMART health, Btrfs usage/error stats, `btrfs-snapshot-*` / `rclone-sync-*` systemd timers, `journald` logs, `rclone` remote configuration status, and Samba file sharing status:
```bash
# Full health report
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml

# Scoped report using tags (base, locales, ntp, sshd, ufw, crypto, snapshot, rclone, samba, runner)
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags crypto,samba
```

---

## 📚 Documentation & Guidelines

- **[macOS User Guide: File Sharing & Snapshots](docs/mac-user-guide.md)**: Connecting from macOS Finder (SMB3), browsing historical Btrfs snapshots natively, and configuring automated `rsync` over SSH backups.
- **[Detailed Ansible Architecture & Roles Guide](docs/ansible.md)**: Deep dive into all playbooks, roles, and storage layout.
- **[GitHub Actions CI/CD & Self-Hosted Runner Setup Guide](docs/github-ci-runner-setup.md)**: Step-by-step setup for the two-stage pipeline, `local-runner` approval gate, and `runner.delarche.fr` VM provisioning.
- **[Test Book & Non-Regression Verification Guide](docs/testing.md)**: Complete catalog of QA, system, firewall, storage, backup, Samba, and chaos test cases (`TC-*`).
- **[Disaster Recovery & Storage Operations Guide](docs/disaster-recovery.md)**: Step-by-step procedures for emergency recovery, file restoration, and LUKS header repair.
- **[AI Agent & Contributor Guidelines](AGENTS.md)**: Language policy (mandatory English), coding standards, and pre-commit rules.

---

## 🛠️ Utility & Test Scripts (`scripts/`)

The repository includes a modular lab integration test runner targeting `debian.delarche.fr`:

- **[`scripts/test-lab.sh`](scripts/test-lab.sh)**: Automated non-regression test runner supporting granular subcommands (`check-prereqs`, `system-and-ufw`, `storage-wipe-and-unlock`, `backup-and-sync`, `samba`, `corner-cases-and-raid1`, `status`, `all`).

---

## 🧪 Code Quality & Pre-commit

All contributions must pass the full pre-commit test suite before committing:

```bash
uv run pre-commit run --all-files
```
Enforces `ansible-lint`, `yamllint`, `shellcheck`, and standard security checks.
