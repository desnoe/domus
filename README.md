# homecode

Infrastructure as Code (IaC) repository for provisioning, hardening, and managing home Debian servers, encrypted storage stacks, and automated cloud backups.

---

## 📌 Features & Architecture Overview

- **Host Provisioning**: Bootstrap minimal Debian systems with automated user management (`ansible_user`), SSH hardening, and sudo rights.
- **Layered Storage Security**:
  - **LUKS**: Block-level encryption with AES-256 (`luks_pass`) on each member device.
  - **Btrfs**: Filesystem management directly on decrypted LUKS containers with native RAID1 mirroring for multi-device pools, declarative subvolumes (`sync` and `snapshots`), transparent zstd compression, and automated fstab integration.
- **Automated Backups**:
  - Daily read-only Btrfs snapshots scheduled via cron at 05:00.
  - Multi-cloud synchronization via `rclone` (OneDrive and S3) scheduled at 04:00.
- **System Hardening**: Baseline configuration covering `iptables` firewall, SSH keys, locales, NTP, and administration tools.

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
All sensitive credentials (`luks_pass`, `user_password`) are encrypted using `ansible-vault`. Ensure your vault password is saved in `~/.ansible_vault_pass` with restricted permissions:

```bash
echo "YOUR_VAULT_PASSWORD" > ~/.ansible_vault_pass
chmod 600 ~/.ansible_vault_pass
```

---

## 🔑 Where Variables and Secrets are Defined

The repository uses a clear variable precedence structure:

| Variable Type | Location | Purpose | Examples |
| :--- | :--- | :--- | :--- |
| **Global Defaults** | `ansible/envs/<env>/group_vars/all/main.yml` | Common across all hosts | `crypto_base_dir`, `ansible_user` |
| **Host Variables** | `ansible/envs/<env>/host_vars/<host>/main.yml` | Device layout & backup targets | `crypto_disks`, `rclone_jobs` |
| **Encrypted Secrets** | `ansible/envs/<env>/host_vars/<host>/vault.yml` | Passphrases & sensitive hashes | `luks_pass`, `user_password` |

### User Password (`user_password`)
The `user_password` variable holds the `sha512crypt` password hash for the primary user account (`ansible_user`):
- It is defined in each host's encrypted **`ansible/envs/<env>/host_vars/<host>/vault.yml`**.
- It is consumed by the **`sudo`** role (`ansible/roles/sudo/tasks/main.yml`) with `no_log: true`.
- An unencrypted reference template is provided in [`ansible/vault.yml.example`](ansible/vault.yml.example).

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

### 1. Bootstrap a Fresh Debian Server
Run against a freshly installed minimal Debian host with root password:
```bash
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/bootstrap.yml -k -K
```

### 2. Full Server Reconfiguration & Storage Unlock
Applies baseline hardening, unlocks LUKS, mounts Btrfs subvolumes, and configures backup crons:
```bash
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/reconfigure-and-decrypt.yml
```

### 3. Unlock Storage After Server Reboot
Quickly unlocks LUKS and mounts partitions without re-running system hardening roles:
```bash
uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/decrypt-only.yml
```

### 4. Wipe & Re-encrypt Storage (Destructive)
Unmounts, wipes existing LUKS containers, re-formats, and re-creates declarative Btrfs subvolumes:
```bash
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/wipe-and-reencrypt.yml
```

### 5. Check Storage and System Health (Read-Only)
Inspects LUKS encryption status, header backup presence, Btrfs allocation/compression, and snapshots:
```bash
uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml
```

---

## 📚 Documentation & Guidelines

- **[Detailed Ansible Architecture & Roles Guide](docs/ansible.md)**: Deep dive into all playbooks, roles, and storage layout.
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
