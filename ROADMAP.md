# Roadmap - Modernization & Hardening of Homecode

This document lists all identified improvement axes for the `homecode` repository, organized into logical phases ordered by priority.

---

## 🎯 Current Priority: Development Tooling & Reproducibility
- [x] Initialize Python virtual environment with `uv` (Python 3.11).
- [x] Pin tool versions (`ansible-core`, `ansible-lint`, `yamllint`, `pre-commit`).
- [x] Define and pin Ansible collection dependencies (`ansible/requirements.yml`).
- [x] Set up `pre-commit` with quality checks (`ansible-lint`, `yamllint`, `shellcheck`, etc.).
- [x] Update `.gitignore` to exclude `.venv/`, `ansible/collections/`, caches, and logs.
- [x] Reset Ansible Vault with secure random keys and restore vault password configuration.
- [x] Establish language policy in `AGENTS.md` (all repository contents in English).

---

## 🔴 Phase 1: Immediate Security & Secrets Management (High Priority)

### 1.1. User password encryption
- [x] Move plaintext password hashes from [`ansible/roles/sudo/tasks/main.yml`](ansible/roles/sudo/tasks/main.yml) into encrypted vault files (`vault.yml`).
- [x] Replace hardcoded hash with variable `password: "{{ user_password }}"` and enforce `no_log: true`.

### 1.2. LUKS encryption & decryption hardening
- [x] Remove temporary plaintext password file creation (`/tmp/encdisk.XXXXXXX`) in [`ansible/roles/crypto/tasks/disk-encrypt.yml`](ansible/roles/crypto/tasks/disk-encrypt.yml) and [`ansible/roles/crypto/tasks/disk-decrypt.yml`](ansible/roles/crypto/tasks/disk-decrypt.yml).
- [x] Feed passphrases directly via standard input piping (`cryptsetup ... --key-file -`) with `no_log: true` to prevent secrets leaking to disk or logs.
- [x] Delete obsolete Jinja template `ansible/roles/crypto/templates/encrypt-pass.j2`.

### 1.3. Sudoers configuration hardening
- [x] Replace direct modification of `/etc/sudoers` with a dedicated drop-in file in `/etc/sudoers.d/99-ansible-wheel`.
- [x] Clean up any legacy `%wheel` rule in `/etc/sudoers`.
- [x] Enforce syntax validation `visudo -cf %s`.

### 1.4. Rsync script safety checks
- [x] In [`scripts/rsync-mac-to-external-hdd.sh`](scripts/rsync-mac-to-external-hdd.sh), add a strict mount point check (`[ -d "$DST_DIR" ]`) to prevent filling internal macOS SSD if external drive is unmounted.
- [x] Replace hardcoded user home paths (`/Users/olivier/`) with `$HOME`.
- [x] Systematically ensure parent log directories exist (`mkdir -p "$LOGDIR"`) prior to rsync execution.


---

## 🟡 Phase 2: Ansible Modernization & Storage Reliability (Medium Priority)

### 2.1. Eliminate deprecated syntax and directives
- [x] Replace deprecated `include:` directives with `ansible.builtin.include_tasks:` in [`ansible/roles/crypto/tasks/main.yml`](ansible/roles/crypto/tasks/main.yml).
- [x] Replace inefficient `with_items` loops on `apt` across all roles (`base`, `apt`, `htop`, `iptables`, `locales`, `ntp`, `vim`, `rclone`) with direct list arguments `name: [...]`.
- [x] Fix role inconsistency in `base`: copy [`ansible/roles/base/files/bashrc.root`](ansible/roles/base/files/bashrc.root) for root instead of standard user bashrc.
- [x] Add `become: true` to service activation task in [`ansible/roles/ntp/tasks/main.yml`](ansible/roles/ntp/tasks/main.yml).
- [x] Migrate fully to FQCNs (`ansible.builtin.*`, `ansible.posix.*`, `community.general.*`).

### 2.2. Compatibility with Debian 11/12+
- [x] Replace obsolete transitional package `btrfs-tools` with `btrfs-progs` in [`ansible/roles/crypto/tasks/main.yml`](ansible/roles/crypto/tasks/main.yml).
- [x] Update `ntp` role to install `systemd-timesyncd` or `chrony` on Debian 12 (Bookworm).

### 2.3. Simplify Btrfs mount logic
- [x] Eliminate temporary mounting and regex subvolume ID parsing in [`ansible/roles/crypto/tasks/fs-mount.yml`](ansible/roles/crypto/tasks/fs-mount.yml).
- [x] Use native standard Btrfs subvolume mounting by name: `opts: "subvol=sync"` and `opts: "subvol=snapshots"`.

### 2.4. Clean up duplicate role execution
- [x] Resolve circular dependency between `rclone` and `crypto` in [`ansible/roles/rclone/meta/main.yml`](ansible/roles/rclone/meta/main.yml) and [`ansible/reconfigure-and-decrypt.yml`](ansible/reconfigure-and-decrypt.yml) to avoid executing crypto role twice.

---

## 🟢 Phase 3: Data Maintenance & Retention

### 3.1. Btrfs snapshot retention policy
- [ ] Implement an automated rotation and cleanup mechanism in [`ansible/roles/crypto/files/snapshot.sh`](ansible/roles/crypto/files/snapshot.sh) (e.g. keep 7 daily, 4 weekly, and prune older snapshots).

### 3.2. Stagger and prune rclone backups
- [ ] Stagger cron execution times across buckets (`rclone_jobs`) instead of triggering all at 04:00.
- [ ] Implement automatic cleanup for old log files in [`ansible/roles/rclone/files/rclone.sh`](ansible/roles/rclone/files/rclone.sh) (30-day retention).

---

## 🔵 Phase 4: Script Refactoring, CI & Documentation

### 4.1. ImageMagick script refactoring
- [ ] Standardize script extension: rename [`scripts/png2jpga4`](scripts/png2jpga4) to `png2jpga4.sh`.
- [x] Extract image formats reliably via `magick identify -format "%m" "$infile"`.
- [x] Add bash strict mode (`set -euo pipefail`) across all scripts.
- [ ] *(Optional)* Consolidate image conversion scripts into a single parameterized CLI utility.

### 4.2. CI automation
- [ ] Set up GitHub Actions workflow (`.github/workflows/lint.yml`) to run `pre-commit` (`ansible-lint`, `yamllint`, `shellcheck`) on pull requests and commits.

### 4.3. Comprehensive repository documentation
- [x] Expand [`README.md`](README.md) and [`docs/ansible.md`](docs/ansible.md):
  - Detailed storage architecture (LUKS $\rightarrow$ LVM $\rightarrow$ Btrfs $\rightarrow$ Snapshots $\rightarrow$ Rclone).
  - Playbook usage guide (`bootstrap.yml`, `reconfigure-and-decrypt.yml`, `decrypt-only.yml`, `wipe-and-reencrypt.yml`).
  - Onboarding guide, vault management, and instructions for provisioning new hosts.
