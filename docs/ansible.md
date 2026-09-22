# Ansible Playbooks, Roles & Architecture Guide

This document describes the structure, variable hierarchy, playbooks, and roles used to manage servers and storage in the `homecode` repository.

---

## 🏗️ Storage & Security Architecture

The infrastructure runs on Debian hosts (`lab` and `live` environments). For hosts in the `crypto` inventory group, storage is structured as an encrypted layered stack:

```mermaid
flowchart TD
    Disk["Physical Block Device (e.g. /dev/sda, /dev/sdb)"] -->|LUKS AES-256 (luks_pass)| LUKS["Decrypted Mapper (/dev/mapper/crypt_sda)"]
    LUKS -->|LVM Physical Volume| VG["LVM Volume Group (vg_data)"]
    VG -->|LVM Logical Volume| LV["LVM Logical Volume (lv_data)"]
    LV -->|Btrfs Filesystem| RootBtrfs["Btrfs Root"]
    RootBtrfs --> SubvolSync["Subvolume: sync (Mounted at ~/crypto/sync)"]
    RootBtrfs --> SubvolSnap["Subvolume: snapshots (Mounted at ~/crypto/snapshots)"]
    SubvolSync -->|Daily Snapshot Cron (05:00)| SubvolSnap
    SubvolSync -->|Daily Rclone Cron (04:00)| Cloud["Cloud Storage (OneDrive, S3)"]
```

---

## 📁 Environments & Inventory Structure

Ansible configurations are partitioned under `ansible/envs/`:

- **`ansible/envs/lab/`**: Testing and development environment.
  - Hosts: `debian.delarche.fr`
- **`ansible/envs/live/`**: Production home infrastructure.
  - Hosts: `kodlin1.delarche.fr`, `zotac.delarche.fr`

Each environment contains:
- `inventory.ini`: Host declarations and group assignments (`crypto`).
- `group_vars/all/main.yml`: Global variables common to all hosts in this environment.
- `host_vars/<hostname>/main.yml`: Public host-specific variables.
- `host_vars/<hostname>/vault.yml`: Encrypted secrets for the host.

---

## 🔑 Variables & Secrets Hierarchy

Understanding where variables are defined is essential for maintaining and provisioning hosts.

### 1. Global Variables (`ansible/envs/<env>/group_vars/all/main.yml`)
Defines shared settings across all hosts:
- `crypto_base_dir`: Base destination path for decrypted mounts (default: `/home/olivier/crypto`).
- `ansible_python_interpreter`: Path to target python interpreter (`/usr/bin/python3`).
- `disable_cloud_init_warnings`: Suppress cloud-init warnings if applicable.

### 2. Host Public Variables (`ansible/envs/<env>/host_vars/<hostname>/main.yml`)
Configures hardware layout and backup targets:
- `luks_dev`: Target block device to encrypt (e.g. `/dev/sda` on live, `/dev/sdb` on lab).
- `luks_name`: Device mapper target name (e.g. `crypt_sda`, `crypt_sdb`).
- `rclone_jobs`: List of cloud remotes and buckets to sync via rclone:
  ```yaml
  rclone_jobs:
    - provider: onedrive
      bucket: ""
    - provider: S3
      bucket: delarche-images
  ```

### 3. Encrypted Secrets (`ansible/envs/<env>/host_vars/<hostname>/vault.yml`)
> [!IMPORTANT]
> All sensitive credentials and secrets are encrypted with `ansible-vault` (AES-256) using password file `~/.ansible_vault_pass`.

Each host's `vault.yml` defines:
- **`luks_pass`**: The disk encryption passphrase used by `cryptsetup` to format or open the LUKS container.
- **`user_password`**: The `sha512crypt` password hash for user `olivier`. This variable is consumed by `ansible/roles/sudo/tasks/main.yml` when creating or updating the user account.

#### Managing Vault Files:
A template is maintained in [`ansible/vault.yml.example`](../ansible/vault.yml.example).

To view secrets:
```bash
uv run ansible-vault view ansible/envs/lab/host_vars/debian.delarche.fr/vault.yml
```

To edit secrets:
```bash
uv run ansible-vault edit ansible/envs/lab/host_vars/debian.delarche.fr/vault.yml
```

To re-encrypt or create a new vault:
```bash
uv run ansible-vault encrypt ansible/envs/live/host_vars/newhost/vault.yml
```

---

## 📜 Playbooks Description & Workflows

### 1. `ansible/bootstrap.yml`
- **Purpose**: Initial bootstrap on a clean, minimal Debian install where only `root` exists.
- **Roles executed**: `apt`, `sudo`, `sshd`.
- **Action**: Installs essential packages (`sudo`), creates user `olivier` with sudo rights via `/etc/sudoers.d/99-ansible-wheel` and password `user_password`, provisions authorized SSH keys, and hardens SSH daemon.
- **Usage**:
  ```bash
  uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/bootstrap.yml -k -K
  ```

### 2. `ansible/reconfigure-and-decrypt.yml`
- **Purpose**: Complete host baseline configuration, unlock encrypted storage, and set up backup cron jobs.
- **Target groups**:
  - `hosts: all`: Executes `apt`, `base`, `htop`, `iptables`, `locales`, `ntp`, `sshd`, `sudo`, `vim`.
  - `hosts: crypto`: Executes `crypto`, `rclone`.
- **Usage**:
  ```bash
  uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/reconfigure-and-decrypt.yml
  ```

### 3. `ansible/decrypt-only.yml`
- **Purpose**: Fast playbook used after a server reboot to unlock and mount encrypted storage without re-running the full system configuration roles.
- **Roles executed**: `crypto` (only decrypts LUKS and mounts Btrfs subvolumes).
- **Usage**:
  ```bash
  uv run ansible-playbook -i ansible/envs/live/inventory.ini ansible/decrypt-only.yml
  ```

### 4. `ansible/wipe-and-reencrypt.yml`
- **Purpose**: Destructive playbook used when provisioning a new drive or resetting storage.
- **Variables**: Sets `wipe: true`.
- **Action**: Unmounts filesystems, wipes previous LUKS headers, re-encrypts the block device, re-creates LVM volume group and logical volumes, creates fresh Btrfs subvolumes (`sync`, `snapshots`), and mounts them.
- **Usage**:
  ```bash
  uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/wipe-and-reencrypt.yml
  ```

---

## 🧩 Roles Breakdown

| Role | Description | Key Managed Files / Services |
| :--- | :--- | :--- |
| **`apt`** | Ensures `sudo` and package tools are installed; upgrades system package cache. | `apt` cache, system packages |
| **`base`** | Installs common utilities (`curl`, `rsync`, `tree`, `wget`, etc.), sets `/etc/hostname`, and installs customized `.bashrc`. | `/etc/hostname`, `~/.bashrc`, `/root/.bashrc` |
| **`crypto`** | Manages the full encrypted storage stack: LUKS format/open via memory/stdin, LVM volumes, Btrfs subvolumes (`sync`, `snapshots`), and daily snapshot cron job. | `/dev/mapper/<luks_name>`, `vg_data`, `lv_data`, `snapshot.sh` |
| **`htop`** | Installs `htop` and user configuration. | `~/.config/htop/htoprc` |
| **`iptables`** | Sets up packet filtering firewall rules and restore handler. | `/etc/network/iptables.up.rules` |
| **`locales`** | Configures system locales (`en_US.UTF-8`, `fr_FR.UTF-8`). | `/etc/default/locale`, `/etc/locale.gen` |
| **`ntp`** | Sets timezone (`Europe/Paris`) and configures time synchronization. | `tzdata`, `ntp` service |
| **`rclone`** | Installs `rclone`, sets up sync wrapper script (`rclone.sh`), and configures cron jobs for scheduled cloud backups. | `rclone.sh`, user crontab |
| **`sshd`** | Hardens SSH configuration (disables root login, disables password authentication) and manages `authorized_keys`. | `/etc/ssh/sshd_config`, `~/.ssh/authorized_keys` |
| **`sudo`** | Creates `wheel` group, configures `/etc/sudoers.d/99-ansible-wheel` (validated via `visudo`), and creates user `olivier` with `user_password`. | `/etc/sudoers.d/99-ansible-wheel`, `/etc/passwd` |
| **`vim`** | Installs `vim` and provisions `.vimrc` for regular user and root. | `~/.vimrc`, `/root/.vimrc` |
