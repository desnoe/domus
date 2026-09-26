# Instructions for AI Agents & Contributors

## 🌐 Language Policy (Mandatory)

- **All repository content MUST be written in English**:
  - Documentation (`README.md`, `ROADMAP.md`, architecture guides, etc.)
  - Code, task names, variable names, and comments in all files (Python, Bash, Ansible YAML, Jinja2, etc.)
  - Git commit messages and pull request descriptions
  - Configuration comments (`.ansible-lint`, `.yamllint`, `ansible.cfg`, etc.)
- **Conversations with users**: Even if conversations or requests with users take place in French (or any other language), all artifacts, commits, and file modifications in this repository must remain strictly in English.

---

## 🛠️ Tooling & Environment Standards

- **Python & Environment**:
  - Always use `uv` for Python virtual environments and dependency management.
  - Python version is pinned to `3.11` via `.python-version`.
  - Dependencies are managed via `pyproject.toml` and locked with `uv.lock`.
  - Execute Python/Ansible commands using `uv run <command>`.

- **Ansible Standards**:
  - `ansible-core` version is pinned in `pyproject.toml`.
  - Collection dependencies are declared with exact versions in `ansible/requirements.yml` and installed locally under `ansible/collections/`.
  - Always prefer Fully Qualified Collection Names (FQCN) such as `ansible.builtin.*`, `ansible.posix.*`, `community.general.*`, `community.crypto.*` (including `become_method: ansible.builtin.su`).
  - Avoid deprecated directives (e.g. use `import_tasks` / `include_tasks` instead of `include`).
  - Quote all file permissions (e.g. `mode: "0644"`, `mode: "0755"`).
  - Pass package lists directly to `ansible.builtin.apt` instead of using `with_items` loops.
  - Prefix all role-internal variables (`defaults`, `set_fact`, `register`) with `<role>_` (e.g. `crypto_*`, `snapshot_*`, `rclone_*`).
  - Place Jinja2 expressions in task `name:` only at the very end of the string (`name[template]`).
  - Always specify `changed_when` on `ansible.builtin.command` / `ansible.builtin.shell` tasks, and include `executable: /bin/bash` with `set -o pipefail` whenever using shell pipes.

- **Secrets & Security**:
  - Never commit plaintext secrets, passwords, or password hashes into version control.
  - Encrypt all secrets using `ansible-vault` with password file `~/.ansible_vault_pass` (enforced by the `check-ansible-vault` pre-commit hook).
  - Reference documented variables in `ansible/vault.yml.example`.

- **Pre-commit & Code Quality**:
  - All contributions must pass `uv run pre-commit run --all-files` with a clean exit code (`0`).
  - Linters enforced at maximum strictness: `ansible-lint --strict` (`profile: production`), `yamllint --strict` (`level: error`), `shellcheck --severity=style`, `check-ansible-vault`, and `pre-commit-hooks` (`v5.0.0`).


---

## 🏗️ Storage & Backup Architecture Standards

- **Storage Pools (`crypto_disks`)**:
  - Every pool in `crypto_disks` must declare `name` (used as Btrfs `LABEL`, LUKS mapper prefix `<name>-<idx>`, and header backup filename), `devices` (list of block devices, preferring `/dev/disk/by-id/...`), and `subvolumes` (mandatory list of absolute mount paths).
  - Do not use implicit base directories (`crypto_base_dir`) or implicit default subvolume lists (`btrfs_default_subvolumes`).
  - Internal Btrfs subvolume names are deterministically derived by flattening the absolute mount path with underscores (`subvol | regex_replace('^/+', '') | regex_replace('/+$', '') | replace('/', '_')`), e.g., `/home/olivier/data/sync` $\rightarrow$ `subvol=home_olivier_data_sync`.
- **Backup & Sync (`snapshot_jobs` & `rclone_jobs`)**:
  - Helper scripts and logs are installed directly in `/home/{{ ansible_user }}/bin` and `/home/{{ ansible_user }}/logs`.
  - Both `snapshot_jobs` (`source`, `destination`) and `rclone_jobs` (`provider`, `destination`, `bucket`) must specify explicit absolute paths.

---

## 📜 Playbook Architecture & Operational Workflows

Playbooks are strictly separated by operational responsibility (no role duplication):
1. `ansible/bootstrap.yml`: Day-0 SSH key & passwordless sudo setup (`apt`, `sudo`, `sshd`), chaining `ansible/system.yml`.
2. `ansible/system.yml`: OS baseline, packages, firewall (`iptables`), time sync (`ntp`), and hardening (`hosts: all`).
3. `ansible/storage-unlock.yml`: Non-destructive post-reboot LUKS unlock, `btrfs device scan`, and subvolume mounting (`hosts: crypto`).
4. `ansible/storage-wipe-and-reset.yml`: Destructive storage reset with interactive `WIPE` confirmation (`hosts: crypto`).
5. `ansible/storage-replace.yml`: Guided Btrfs RAID1 member disk replacement (`hosts: crypto`), supporting auto-detection of unformatted/closed disks across multiple pools or targeted replacement via `-e replace_pool=<pool> -e replace_index=<idx>`.
6. `ansible/backup.yml`: Deploys `snapshot` and `rclone` scripts and crontabs (`hosts: crypto`).
7. `ansible/status.yml`: Read-only health reporting across `crypto`, `snapshot`, and `rclone` roles (supports `--tags crypto|snapshot|rclone`).

---

## 🧪 Mandatory Non-Regression Verification Protocol (Lab)

Whenever modifying storage (`crypto`), backup (`snapshot`, `rclone`), or system roles/playbooks, agents MUST execute a full multi-order non-regression test campaign against `ansible/envs/lab/inventory.ini` (`debian.delarche.fr`):
1. **Idempotence & Baseline Chaining**: Run `system.yml` (twice), `bootstrap.yml`, and `status.yml`.
2. **Wipe, Unlock & Backup Interleaving**: Run `storage-wipe-and-reset.yml -e confirm_wipe=yes` $\rightarrow$ `storage-unlock.yml` (verify `changed=0`) $\rightarrow$ `backup.yml` (twice, plus manual `snapshot.sh` execution) $\rightarrow$ `status.yml`.
3. **Simulated Reboot / Lock Cycle**: Unmount all Btrfs subvolumes and close all LUKS mappers on the lab host, run `status.yml` (locked state), then run `storage-unlock.yml` and verify all subvolumes, test files, and snapshots are intact.
4. **Sequential Disk-by-Disk RAID1 Replacement**: Test `storage-replace.yml` sequentially on each member index (`0`, `1`, ...) of every pool (covering both active online replacement via `-e replace_pool=<pool> -e replace_index=<idx>` and auto-detected replacement of a wiped/closed disk), verifying zero data loss and clean `status.yml` / `storage-unlock.yml` execution after each replacement.
