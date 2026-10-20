# 🧪 Test Book & Non-Regression Verification Guide

This document centralizes all static, functional, security, and chaos/corner-case test procedures for the `domus` repository. Every test case is formatted as an actionable Test Card with deterministic commands and pass criteria validated against the `lab` environment (`debian.delarche.fr`).

---

## 🗺️ Test Architecture & Domains Overview

```mermaid
flowchart LR
    QA["1. Static Quality Gates (TC-QA-01..02)"] --> SYS["2. System, Locales, NTP & SSHD (TC-SYS-01..05)"]
    SYS --> FW["3. UFW Dual-Stack Firewall (TC-FW-01..03)"]
    FW --> OBS["4. Unified Observability & Dashboards (TC-OBS-01..03)"]
    OBS --> STO["5. LUKS & Btrfs RAID1 Storage (TC-STO-01..08)"]
    STO --> BAK["6. Systemd Rclone & Snapshots (TC-BAK-01..07)"]
    BAK --> SMB["7. Samba & Shadow Copies (TC-SMB-01..07)"]
    SMB --> ADV["8. Advanced Corner Cases & Chaos (TC-ADV-01..06)"]
    ADV --> RUN["9. Master Non-Regression Checklist"]
```

| Domain Prefix | Scope | Playbooks / Components Covered | Test Count |
| :--- | :--- | :--- | :--- |
| **`TC-QA`** | Static Quality Gates & Linting | `.pre-commit-config.yaml`, `.ansible-lint`, `.yamllint`, `shellcheck`, `vault.yml`, `dashboards/` | 2 |
| **`TC-SYS`** | System Baseline, Locales, Time, SSH & S.M.A.R.T. | `bootstrap.yml`, `system.yml`, `apt`, `base`, `locales`, `ntp`, `sshd`, `sudo`, `vim`, `htop`, `smartctl` | 5 |
| **`TC-FW`** | Dual-Stack Firewall (`ufw`) | `system.yml`, `ufw` role, `iptables` / `ip6tables` kernel chains | 3 |
| **`TC-OBS`** | Unified Observability, Telemetry & Dashboards | `system.yml`, `status.yml`, `alloy` role, `dashboards/`, `scripts/dashboards.sh`, `scripts/verify_live.py` | 3 |
| **`TC-STO`** | Encrypted Storage & Btrfs RAID1 | `storage-wipe-and-reset.yml`, `storage-unlock.yml`, `storage-replace.yml`, `crypto` role | 8 |
| **`TC-BAK`** | Cloud Sync & Btrfs Snapshots (`systemd`) | `backup.yml`, `status.yml`, `rclone` role, `snapshot` role | 7 |
| **`TC-SMB`** | Samba File Sharing & Shadow Copies | `system.yml`, `status.yml`, `samba` role, `vfs_shadow_copy2`, `smbclient` | 7 |
| **`TC-ADV`** | Advanced Corner Cases & Chaos Sequences | Unmounted pool guards, live unmount kill-switch, kernel sandboxing, ISO-week pruning, header recovery | 6 |

---

## 1. Static Quality Gates & Linting (`TC-QA`)

### `TC-QA-01` — Full Pre-Commit Hook Suite
- **Objective**: Verify that all YAML, Ansible, Bash, Go, and Vault files comply with strict production linting and validation rules before any deployment.
- **Preconditions**: Local Python 3.11 `.venv` initialized via `uv sync` and Ansible collections installed in `ansible/collections/`.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run pre-commit run --all-files
  ```
- **Expected Result**:
  - Exit code `0`.
  - All 21 hooks report `Passed` (`ansible-lint --strict` with `profile: production`, `yamllint --strict`, `shellcheck --severity=style`, `check-ansible-vault`, `go-fmt`, `go-vet`, `go-staticcheck`, `go-test`, `dashboards-validate`, and `pre-commit-hooks v5.0.0`).

---

### `TC-QA-02` — Playbook Syntax & Vault Encryption Verification
- **Objective**: Ensure all 11 operational playbooks parse cleanly across `lab` and `live` inventories and no `vault.yml` file is stored in plaintext.
- **Preconditions**: `~/.ansible_vault_pass` present (`mode 0600`).
- **Commands**:
  ```bash
  for pb in ansible/*.yml; do
    [ "$(basename "$pb")" = "requirements.yml" ] && continue
    ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini --syntax-check "$pb"
  done
  head -n 1 ansible/envs/*/host_vars/*/vault.yml
  ```
- **Expected Result**:
  - Every playbook prints `playbook: ansible/<name>.yml` with exit code `0`.
  - Every `vault.yml` header starts with `$ANSIBLE_VAULT;1.1;AES256`.

---

## 2. System Baseline, Locales, Time/NTP & SSHD (`TC-SYS`)

### `TC-SYS-01` — Day-0 Bootstrap & System Baseline Idempotence
- **Objective**: Verify that `system.yml` (and `bootstrap.yml` chaining `system.yml`) converges cleanly and achieves `changed=0` on the second execution.
- **Preconditions**: Lab VM (`debian.delarche.fr`) reachable over SSH.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/system.yml
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/system.yml
  ```
- **Expected Result**:
  - Second run completes with `changed=0, unreachable=0, failed=0`.

---

### `TC-SYS-02` — Hybrid English UI / French Regional Units & French Keyboard
- **Objective**: Verify that the system UI and logs remain in English (`en_US.UTF-8`) while date, metric units, paper size, currency, and console keyboard use French standards (`fr_FR.UTF-8`, `fr`/`latin9`), and `/etc/default/locale` is a symlink to `/etc/locale.conf`.
- **Preconditions**: `system.yml` applied.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags locales
  ssh olivier@debian.delarche.fr "readlink -f /etc/default/locale && locale -a | grep -E 'en_US|fr_FR'"
  ```
- **Expected Result**:
  - `readlink -f /etc/default/locale` outputs `/etc/locale.conf`.
  - `LANG=en_US.UTF-8` and `LC_TIME=fr_FR.UTF-8`, `LC_MEASUREMENT=fr_FR.UTF-8`, `LC_PAPER=fr_FR.UTF-8`, `LC_MONETARY=fr_FR.UTF-8`.
  - `/etc/default/keyboard` contains `XKBLAYOUT="fr"` and `XKBVARIANT="latin9"`.

---

### `TC-SYS-03` — Timezone (`Europe/Paris`), Chrony NTP & Leap-Second Slew
- **Objective**: Confirm `Europe/Paris` timezone (automatic CET/CEST transitions), `fr.pool.ntp.org` synchronization, and monotonic leap-second slewing (`leapsecmode slew`).
- **Preconditions**: `system.yml` applied.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags ntp
  ssh olivier@debian.delarche.fr "grep -E '^(pool|leapseclist|leapsecmode)' /etc/chrony/chrony.conf"
  ```
- **Expected Result**:
  - `Time zone: Europe/Paris`, `System clock synchronized: yes`, `NTP service: active`.
  - `chronyc tracking` reports `Leap status : Normal`.
  - `/etc/chrony/chrony.conf` contains `pool fr.pool.ntp.org iburst`, `leapseclist /usr/share/zoneinfo/leap-seconds.list`, and `leapsecmode slew`.

---

### `TC-SYS-04` — SSHD Multi-Key Auto-Discovery & Authentication Hardening
- **Objective**: Verify that all local `~/.ssh/*.pub` keys (`id_ecdsa.pub`, `id_rsa.pub`, etc.) are deployed with `exclusive: true`, password/root login is disabled, and each key can authenticate independently.
- **Preconditions**: `system.yml` applied.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags sshd
  for key in ~/.ssh/*.pub; do
    priv="${key%.pub}"
    ssh -i "$priv" -o IdentitiesOnly=yes -o BatchMode=yes olivier@debian.delarche.fr "echo Auth OK with $(basename "$priv")"
  done
  ```
- **Expected Result**:
  - `sshd -T` reports `passwordauthentication no`, `permitrootlogin no`, `pubkeyauthentication yes`.
  - Every local key prints `Auth OK with <key_name>`.

---

### `TC-SYS-05` — S.M.A.R.T. Hardware Health Monitoring & `smartctl_exporter` Verification
- **Objective**: Verify that `smartmontools` and `smartctl_exporter` are deployed, the exporter service is running sandboxed on loopback (`127.0.0.1:9633`), and `status.yml --tags smartctl` reports S.M.A.R.T. health cleanly.
- **Preconditions**: `system.yml` applied.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "
  sudo systemctl is-active smartctl_exporter | grep -q '^active$'
  curl -fsS http://127.0.0.1:9633/metrics | grep -q 'smartctl_'
  "
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags smartctl
  ```
- **Expected Result**:
  - `systemctl is-active smartctl_exporter` reports `active`.
  - HTTP probe to `http://127.0.0.1:9633/metrics` returns valid Prometheus metrics prefixed with `smartctl_`.
  - `status.yml` prints the `S.M.A.R.T. MONITORING REPORT` with `Service: smartctl_exporter (active)` and positive device count.

---

## 3. Dual-Stack Firewall (`ufw`) 3-Step Non-Regression (`TC-FW`)

### `TC-FW-01` — Service & Dual-Stack Netfilter Kernel State
- **Objective**: Verify that `ufw` is active, enabled at boot, and enforces default-deny incoming filtering in both IPv4 (`iptables`) and IPv6 (`ip6tables`).
- **Preconditions**: `system.yml` applied.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags ufw
  ssh olivier@debian.delarche.fr "sudo systemctl is-enabled ufw && sudo iptables -S ufw-user-input && sudo ip6tables -S ufw6-user-input"
  ```
- **Expected Result**:
  - `Status: active`, `Default: deny (incoming), allow (outgoing), deny (routed)`.
  - `ufw-user-input` and `ufw6-user-input` contain `-p tcp --dport 22 -j ACCEPT` and no unauthorized ports.

---

### `TC-FW-02` — Live Positive & Negative Network Filtering Probe
- **Objective**: Prove with real network traffic that loopback traffic (`lo`) and SSH (`22/tcp`) are allowed while unauthorized external TCP ports (`9999/tcp`) are dropped.
- **Preconditions**: `ufw` active on `debian.delarche.fr`.
- **Commands**:
  ```bash
  # 1. Start ephemeral listener on port 9999 on target, test loopback, then test from controller
  ssh olivier@debian.delarche.fr "nohup python3 -m http.server 9999 --bind 0.0.0.0 >/dev/null 2>&1 & echo \$! > /tmp/http9999.pid; sleep 1; curl -fsS http://127.0.0.1:9999 >/dev/null && echo LOOPBACK_OK"
  ! nc -zv -G 2 debian.delarche.fr 9999 && echo EXTERNAL_9999_BLOCKED_OK
  nc -zv -G 2 debian.delarche.fr 22 && echo EXTERNAL_22_ALLOWED_OK
  ssh olivier@debian.delarche.fr "kill \$(cat /tmp/http9999.pid) 2>/dev/null || true; rm -f /tmp/http9999.pid"
  ```
- **Expected Result**:
  - `LOOPBACK_OK`, `EXTERNAL_9999_BLOCKED_OK` (`Operation timed out` / dropped), and `EXTERNAL_22_ALLOWED_OK`.

---

### `TC-FW-03` — Firewall Drift / Disable Remediation
- **Objective**: Verify that if `ufw` is manually disabled on a host, running `system.yml` detects the drift (`changed=1`) and re-enables the firewall automatically.
- **Preconditions**: `system.yml` applied.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "sudo ufw disable"
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/system.yml
  ssh olivier@debian.delarche.fr "sudo ufw status verbose"
  ```
- **Expected Result**:
  - Task `ufw : Enable UFW firewall` reports `changed: [debian.delarche.fr]`.
  - `sudo ufw status verbose` returns `Status: active`.

---

## 4. Unified Observability & Telemetry (`TC-OBS`)

### `TC-OBS-01` — Alloy Service Deployment, Sandboxing & HTTP Readiness Probe
- **Objective**: Verify that Grafana Alloy daemon is properly installed, bound to systemd sandboxing limits (`Nice=15`, `MemoryMax=512M`, `ProtectSystem=strict`), running actively, and responding to HTTP readiness probes.
- **Preconditions**: `system.yml` applied.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "
  sudo systemctl is-active alloy | grep -q '^active$'
  sudo systemctl show alloy.service -p Nice -p MemoryMax -p ProtectSystem
  curl -fsS http://127.0.0.1:12345/-/ready
  "
  ```
- **Expected Result**:
  - `systemctl is-active alloy` returns `active`.
  - `Nice=15`, `MemoryMax=536870912` (512M), `ProtectSystem=strict`.
  - `curl` probe to `http://127.0.0.1:12345/-/ready` exits with `0` and body `ready`.

---

### `TC-OBS-02` — Alloy LAN Web Interface UFW Firewall Verification & Status Report
- **Objective**: Confirm that the Alloy internal UI port (`12345/tcp`) is explicitly allowed through UFW for the LAN subnet (`alloy_allowed_subnet`), and verify that `status.yml --tags alloy` reports daemon health cleanly.
- **Preconditions**: `system.yml` applied.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "sudo ufw status verbose | grep -E '12345/tcp.*ALLOW IN'"
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags alloy
  ```
- **Expected Result**:
  - UFW rules contain `12345/tcp ALLOW IN <alloy_allowed_subnet>`.
  - `status.yml` executes with exit code `0` and prints the `GRAFANA ALLOY STATUS REPORT` with `Active State: active` and `HTTP Readiness: 200`.

---

### `TC-OBS-03` — Declarative Dashboards Validation & Live Telemetry Verification
- **Objective**: Verify that the Go dashboard generator (`dashboards/`) passes unit tests and schema validation against Grafana Cloud, and that all 121 panel and variable queries across `domus-host`, `domus-storage`, `domus-backups`, and `domus-alloy-health` return live telemetry and render populated PNG snapshots.
- **Preconditions**: `gcx` authenticated with `--context domus`.
- **Commands**:
  ```bash
  ./scripts/dashboards.sh validate
  ./scripts/dashboards.sh test-live
  ```
- **Expected Result**:
  - `validate` passes Go unit tests (`go test -v ./...`) and `gcx --context domus resources validate` with `0` errors.
  - `test-live` deploys the ephemeral preview folder, passes all 121 live Mimir/Loki telemetry and snapshot rendering checks, and automatically purges the preview folder on exit.

---

## 5. Encrypted Storage (`crypto`) & Btrfs RAID1 Lifecycle (`TC-STO`)

### `TC-STO-01` — Destructive Wipe & Multi-Pool RAID1 Initialization
- **Objective**: Validate full LUKS formatting, header backup creation (`0400`), multi-device Btrfs RAID1 creation (`-d raid1 -m raid1`), and flattened subvolume mounting across all pools (`data` and `backup`).
- **Preconditions**: Lab VM with 4 virtual disks (`ata-debian-1..4`).
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-wipe-and-reset.yml -e confirm_wipe=yes
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags crypto
  ```
- **Expected Result**:
  - `/var/backups/luks/luks-header-{data,backup}-{0,1}.img` present with mode `0400`.
  - `btrfs filesystem df /home/olivier/data/sync` and `/home/olivier/backup/sync` show `Data, RAID1` and `Metadata, RAID1`.
  - `/home/olivier/data/sync`, `/home/olivier/data/snapshots`, `/home/olivier/backup/sync`, and `/home/olivier/backup/snapshots` are mounted with `rw,async,compress=zstd:1,noatime,user_subvol_rm_allowed`.

---

### `TC-STO-02` — Post-Reboot Unlock Idempotence
- **Objective**: Verify that running `storage-unlock.yml` when pools are already unlocked and mounted is 100% idempotent (`changed=0`).
- **Preconditions**: Pools unlocked and mounted.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-unlock.yml
  ```
- **Expected Result**:
  - `changed=0, unreachable=0, failed=0`.

---

### `TC-STO-03` — Simulated Reboot / Full Lock & Unlock Cycle
- **Objective**: Simulate a server reboot by unmounting all Btrfs subvolumes and closing all LUKS mappers, verify `status.yml` handles the locked state gracefully, and unlock via `storage-unlock.yml` with zero data loss.
- **Preconditions**: Pools unlocked and mounted.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "echo 'REBOOT_SURVIVAL_TEST' > /home/olivier/data/sync/reboot_test.txt && sync && sudo umount /home/olivier/data/sync /home/olivier/data/snapshots /home/olivier/backup/sync /home/olivier/backup/snapshots && sudo cryptsetup luksClose data-0 && sudo cryptsetup luksClose data-1 && sudo cryptsetup luksClose backup-0 && sudo cryptsetup luksClose backup-1"
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags crypto
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-unlock.yml
  ssh olivier@debian.delarche.fr "cat /home/olivier/data/sync/reboot_test.txt && rm -f /home/olivier/data/sync/reboot_test.txt"
  ```
- **Expected Result**:
  - Locked `status.yml` reports mappers as `LOCKED / CLOSED` without failing (`failed=0`).
  - `storage-unlock.yml` unlocks all 4 mappers, mounts all subvolumes, and `cat` outputs `REBOOT_SURVIVAL_TEST`.

---

### `TC-STO-04` — Online Targeted RAID1 Disk Replacement (`storage-replace.yml`)
- **Objective**: Test online replacement of a specific pool member (`-e replace_pool=<pool> -e replace_index=<idx>`) across `data` and `backup` pools without data loss.
- **Preconditions**: Pools unlocked and mounted.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-replace.yml -e replace_pool=data -e replace_index=1 -e confirm_replace=yes
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags crypto
  ```
- **Expected Result**:
  - `btrfs replace status` reports `0 write errs, 0 uncorr. read errs`.
  - Both member devices (`devid 1` and `devid 2`) are active in `btrfs filesystem show` with `0` Btrfs device errors.

---

### `TC-STO-05` — Non-Interactive Pool Removal & Verification (`storage-remove.yml`)
- **Objective**: Verify non-interactive pool decommissioning via `storage-remove.yml -e remove_pool=backup -e confirm_remove=yes`, confirming full unmounting, mapper closing, metadata/header wipe, `/etc/fstab` cleanup, and header backup removal.
- **Preconditions**: `backup` pool is unlocked and mounted on the lab host.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-remove.yml -e remove_pool=backup -e confirm_remove=yes
  ssh olivier@debian.delarche.fr "mountpoint -q /home/olivier/backup || echo 'UNMOUNTED'; ls -l /dev/mapper/backup-* 2>/dev/null || echo 'CLOSED'; sudo cryptsetup isLuks /dev/sdd || echo 'WIPED_SDD'; sudo cryptsetup isLuks /dev/sde || echo 'WIPED_SDE'; grep -c 'LABEL=backup' /etc/fstab || echo 'FSTAB_CLEAN'"
  ```
- **Expected Result**:
  - `/home/olivier/backup` is unmounted (`UNMOUNTED`).
  - `/dev/mapper/backup-0` and `backup-1` are closed (`CLOSED`).
  - Physical devices `/dev/sdd` and `/dev/sde` are no longer valid LUKS containers (`isLuks` fails, reporting `WIPED_SDD` and `WIPED_SDE`).
  - `/etc/fstab` contains 0 entries for `LABEL=backup` (`FSTAB_CLEAN`).
  - Stale header files `/var/backups/luks/luks-header-backup-*.img` are absent.

---

### `TC-STO-06` — Member Disk Removal & Live Index Compaction (`storage-disk-remove.yml`)
- **Objective**: Verify non-interactive member disk removal from a 2-disk RAID1 pool, including automated RAID1 $\rightarrow$ single/dup profile conversion, Btrfs device eviction, LUKS closure and wiping of the physical disk, and live mapper index compaction via `dmsetup rename`.
- **Preconditions**: `backup` pool is unlocked and mounted with 2 member disks (`/dev/sdd` as `backup-0`, `/dev/sde` as `backup-1`).
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-disk-remove.yml -e pool=backup -e disk_index=0 -e confirm_remove=yes
  ssh olivier@debian.delarche.fr "sudo btrfs filesystem df /home/olivier/backup; sudo btrfs filesystem show /home/olivier/backup; ls -l /dev/mapper/backup-*; sudo cryptsetup isLuks /dev/sdd || echo 'WIPED_SDD'"
  ```
- **Expected Result**:
  - `backup` pool remains online and mounted.
  - Data profile is converted to `single`, metadata to `dup`.
  - Member count in Btrfs filesystem is `1`.
  - Removed disk `/dev/sdd` is no longer a LUKS container (`isLuks` fails, reporting `WIPED_SDD`).
  - Surviving device `/dev/sde` has been live-compacted from `backup-1` to `backup-0` (`/dev/mapper/backup-0` is active; `/dev/mapper/backup-1` does not exist).
  - Header file `/var/backups/luks/luks-header-backup-0.img` is present, while index 1 is deleted.

---

### `TC-STO-07` — Member Disk Addition & RAID1 Conversion (`storage-disk-add.yml`)
- **Objective**: Verify non-interactive member disk addition to a single-disk pool, including LUKS formatting, header backup creation, Btrfs device addition, and automated single $\rightarrow$ RAID1 profile conversion balance.
- **Preconditions**: `backup` pool is online with 1 disk (`backup-0` on `/dev/sde`), and `/dev/sdd` is wiped.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-disk-add.yml -e pool=backup -e device=/dev/sdd -e confirm_add=yes
  ssh olivier@debian.delarche.fr "sudo btrfs filesystem df /home/olivier/backup; sudo btrfs filesystem show /home/olivier/backup; ls -l /dev/mapper/backup-*"
  ```
- **Expected Result**:
  - `backup` pool has 2 active member devices (`devid 1` and `devid 2`).
  - Both `/dev/mapper/backup-0` and `/dev/mapper/backup-1` are active and valid LUKS containers.
  - Btrfs filesystem profiles report `Data, RAID1` and `Metadata, RAID1`.
  - Both header backups `/var/backups/luks/luks-header-backup-{0,1}.img` exist with mode `0400`.
  - Scrub reports 0 errors (`sudo btrfs scrub start -B /home/olivier/backup`).

---

### `TC-STO-08` — Automated Periodic Btrfs Scrub Verification (`btrfs-scrub-*.timer` & `.service`)
- **Objective**: Verify that monthly Btrfs scrub systemd timer units are installed, enabled, and active across all storage pools, that scrub operations can be executed on demand under idle I/O priority, and that scrub results are recorded cleanly in systemd journal logs.
- **Preconditions**: Storage pools unlocked and mounted.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "
  sudo systemctl is-enabled btrfs-scrub-data.timer | grep -q '^enabled$'
  sudo systemctl is-active btrfs-scrub-data.timer | grep -q '^active$'
  sudo systemctl start btrfs-scrub-data.service
  sudo btrfs scrub status /home/olivier/data/sync | grep -E 'Scrub (started|finished)'
  "
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags crypto
  ```
- **Expected Result**:
  - `btrfs-scrub-data.timer` and `btrfs-scrub-backup.timer` are enabled and active.
  - Manual execution of `btrfs-scrub-data.service` succeeds with `status: finished` and 0 errors.
  - `status.yml --tags crypto` displays the Btrfs scrub status and active timer details under `BTRFS SCRUB & PERIODIC VERIFICATION`.

---

## 6. Cloud Sync (`rclone`) & Btrfs Snapshots (`snapshot`) via `systemd` (`TC-BAK`)

### `TC-BAK-01` — `backup.yml` Deployment, Official `rclone` Package & Idempotence
- **Objective**: Verify that `backup.yml` installs the current upstream `rclone` `.deb` package, copies `~/.config/rclone/rclone.conf` (`0600`, `no_log: true`), enables all `systemd` timers, and achieves `changed=0` on the second run.
- **Preconditions**: Pools unlocked and mounted; local `~/.config/rclone/rclone.conf` present.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/backup.yml
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/backup.yml
  ```
- **Expected Result**:
  - Second run completes with `changed=0, failed=0`.
  - `rclone --version` on the remote host matches `https://downloads.rclone.org/version.txt` (`v1.75.1+`).
  - `/home/olivier/.config/rclone/rclone.conf` has mode `0600`.

---

### `TC-BAK-02` — Live `OneDrive:/Share/50ans` Cloud Synchronization
- **Objective**: Trigger `rclone-sync-onedrive-share-50ans.service` and verify that the 8 reference files from `OneDrive:/Share/50ans` are downloaded with exact byte sizes.
- **Preconditions**: `backup.yml` applied and `/home/olivier/data/sync` mounted.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "sudo systemctl start rclone-sync-onedrive-share-50ans.service && rclone ls /home/olivier/data/sync/OneDrive/Share/50ans | sort -k2"
  ```
- **Expected Result**:
  ```text
    2967925 001_20092025_IMG_4422.jpg
    4308511 002_20092025_IMG_4431.jpg
    4674295 003_20092025_IMG_4436.jpg
    4671957 004_20092025_IMG_4438.jpg
    3986394 005_20092025_IMG_4445.jpg
   14961134 006_20092025__MGL9738.jpg
   16677274 007_20092025__MGL9740.jpg
   21642360 008_20092025__MGL9742.jpg
  ```

---

### `TC-BAK-03` — Live Read-Only Btrfs Snapshot Creation (`btrfs-snapshot-data.service`)
- **Objective**: Trigger `btrfs-snapshot-data.service` and verify that a read-only Btrfs subvolume snapshot (`YYYY-MM-DD--HH-MM-SS`) is created in `/home/olivier/data/snapshots/` containing the synced OneDrive files.
- **Preconditions**: `TC-BAK-02` completed.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "sudo systemctl start btrfs-snapshot-data.service && SNAP=\$(ls -td /home/olivier/data/snapshots/20* | head -n 1) && sudo btrfs property get \"\$SNAP\" ro && rclone ls \"\$SNAP/OneDrive/Share/50ans\" | sort -k2"
  ```
- **Expected Result**:
  - `btrfs property get` outputs `ro=true`.
  - `rclone ls` inside the snapshot outputs the exact 8 files from `TC-BAK-02`.

---

### `TC-BAK-04` — Observability & Status Reporting (`status.yml`)
- **Objective**: Verify that `status.yml --tags snapshot,rclone` reports `rclone` version, `rclone.conf` mode `0600`, configured vs missing providers (`OneDrive: Configured`, `S3: MISSING`), configured sync jobs, active systemd timers, and recent `journald` logs.
- **Preconditions**: `backup.yml` applied.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/status.yml --tags snapshot,rclone
  ```
- **Expected Result**:
  - Displays `OneDrive: Configured` and `S3: MISSING (run rclone config)`, `btrfs-snapshot-data.timer`, `rclone-sync-*.timer`, and `journalctl` transfer/snapshot logs.

---

### `TC-BAK-05` — Multi-Destination Cloud Sync & Dynamic Mount Unit Resolution
- **Objective**: Trigger `rclone-sync-onedrive-share-50ans-backup.service` (syncing `OneDrive:Share/50ans` to `/home/olivier/backup/sync/onedrive/50ans`) and verify the 8 reference files, verify that `systemd-escape` resolves `home-olivier-backup-sync.mount`, and verify that `btrfs-snapshot-backup.service` creates a valid read-only snapshot in `/home/olivier/backup/snapshots/`.
- **Preconditions**: `backup.yml` applied and pools mounted.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "set -e
  sudo systemctl start rclone-sync-onedrive-share-50ans-backup.service
  count=\$(rclone ls /home/olivier/backup/sync/onedrive/50ans | wc -l | tr -d ' ')
  test \"\${count}\" -eq 8
  unit=\$(systemctl show -p Requisite --value rclone-sync-onedrive-share-50ans-backup.service)
  test \"\${unit}\" = 'home-olivier-backup-sync.mount'
  systemctl is-active \"\${unit}\" | grep -q 'active'
  sudo systemctl start btrfs-snapshot-backup.service
  SNAP=\$(ls -td /home/olivier/backup/snapshots/20* | head -n 1)
  sudo btrfs property get \"\$SNAP\" ro | grep -q 'ro=true'
  rclone ls \"\$SNAP/onedrive/50ans\" | wc -l | tr -d ' ' | grep -q '^8$'
  "
  ```
- **Expected Result**:
  - Exactly 8 reference files synced to the secondary pool under `/home/olivier/backup/sync/onedrive/50ans`.
  - `systemctl show -p Requisite` matches `home-olivier-backup-sync.mount`.
  - Read-only snapshot created under `/home/olivier/backup/snapshots/` containing all 8 files.

---

### `TC-BAK-06` — Strict Schema & Overlapping Target Directory Rejection (Negative Suite)
- **Objective**: Verify that `backup.yml` pre_tasks block invalid `rclone_jobs` definitions at the playbook threshold before touching systemd or filesystem state.
- **Preconditions**: Lab inventory configured.
- **Test Matrix**:
  1. Duplicate job name.
  2. Nested / overlapping target directories (`/home/olivier/data/sync/a` and `/home/olivier/data/sync/a/b`).
  3. Absolute `destination.path` (`/test`).
  4. Path traversal in `destination.path` (`../../etc`).
  5. Invalid name regex (e.g. uppercase / underscores `bad_Name`).
  6. Unknown/disallowed configuration keys.
  7. Legacy flat schema (`provider:` / `bucket:` at root).
  8. `destination.mount` not declared in `crypto_disks[*].subvolumes`.
- **Expected Result**:
  - Every invalid definition fails immediately with `fatal: [debian.delarche.fr]: FAILED! => ... rclone_jobs validation failed: ...`.
  - No changes applied (`changed=0`).

---

### `TC-BAK-07` — Automatic Cleanup of Orphaned Systemd Units
- **Objective**: Verify that creating an unmanaged systemd unit file (`rclone-sync-orphan-test.{service,timer}`) and running `backup.yml` automatically stops, disables, and deletes the orphan unit without affecting declared timers.
- **Preconditions**: `backup.yml` applied.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "set -e
  cat << 'UNIT' | sudo tee /etc/systemd/system/rclone-sync-orphan-test.service >/dev/null
  [Unit]
  Description=Orphan test service
  [Service]
  Type=oneshot
  ExecStart=/bin/true
  UNIT
  cat << 'UNIT' | sudo tee /etc/systemd/system/rclone-sync-orphan-test.timer >/dev/null
  [Unit]
  Description=Orphan test timer
  [Timer]
  OnCalendar=*-*-* 04:00:00
  Unit=rclone-sync-orphan-test.service
  [Install]
  WantedBy=timers.target
  UNIT
  sudo systemctl daemon-reload
  sudo systemctl enable --now rclone-sync-orphan-test.timer
  "
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/backup.yml
  ssh olivier@debian.delarche.fr "set -e
  test ! -f /etc/systemd/system/rclone-sync-orphan-test.service
  test ! -f /etc/systemd/system/rclone-sync-orphan-test.timer
  ! systemctl is-active rclone-sync-orphan-test.timer >/dev/null 2>&1
  "
  ```
- **Expected Result**:
  - Orphan unit files are deleted from `/etc/systemd/system/`.
  - Orphan timer is stopped and disabled.
  - All declared rclone timers remain active.

---

## 7. Samba File Sharing & Shadow Copies (`TC-SMB`)

### `TC-SMB-01` — Service Health & Port 445/TCP Listener
- **Objective**: Verify that the `smbd` systemd service is active and listening exclusively on port `445/tcp` (NetBIOS port 139 disabled).
- **Preconditions**: `ansible/system.yml --tags samba` applied.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "set -e
  systemctl is-active smbd
  ss -tlnp sport = :445 | grep -q ':445'
  "
  ```
- **Expected Result**:
  - `systemctl is-active smbd` outputs `active`.
  - `ss -tlnp` confirms port 445 is bound by `smbd`.

---

### `TC-SMB-02` — Authenticated SMB3 Connection & Directory Listing
- **Objective**: Verify that user `olivier` can authenticate against Samba using the vault-provisioned password over SMB3 and list files in the `data` share.
- **Preconditions**: `TC-SMB-01` passed.
- **Commands**:
  ```bash
  SAMBA_PASS="$(uv run ansible-vault view ansible/envs/lab/host_vars/debian.delarche.fr/vault.yml | grep '^samba_pass:' | sed 's/^samba_pass: *//' | tr -d '\"')"
  smbclient "//debian.delarche.fr/data" -U "olivier%${SAMBA_PASS}" --max-protocol=SMB3 -c "ls"
  ```
- **Expected Result**:
  - `smbclient` connects successfully without prompt.
  - Returns directory listing including `.`, `..`, and existing synchronized files.

---

### `TC-SMB-03` — Shadow Copies Configuration & Snapshot Integration
- **Objective**: Verify that `vfs_shadow_copy2` is loaded, configured with `shadow:format = %Y-%m-%d--%H-%M-%S`, and maps snapshots on disk to SMB shadow copies.
- **Preconditions**: `TC-BAK-03` completed (at least one snapshot exists in `/home/olivier/data/snapshots`).
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "set -e
  testparm -s 2>/dev/null | grep 'vfs objects' | grep -q 'shadow_copy2'
  testparm -s 2>/dev/null | grep -q 'shadow:format = %Y-%m-%d--%H-%M-%S'
  count=\"\$(find /home/olivier/data/snapshots -maxdepth 1 -mindepth 1 -type d | wc -l | tr -d ' ')\"
  test \"\${count}\" -ge 1
  "
  ```
- **Expected Result**:
  - `testparm` outputs show `shadow_copy2` in `vfs objects` and proper format.
  - At least one valid Btrfs snapshot exists in the destination directory.

---

### `TC-SMB-04` — Read-Only Share Permission Enforcement
- **Objective**: Verify that write operations over SMB to the `data` share are strictly rejected by Samba (`read only = yes`).
- **Preconditions**: `TC-SMB-02` passed.
- **Commands**:
  ```bash
  SAMBA_PASS="$(uv run ansible-vault view ansible/envs/lab/host_vars/debian.delarche.fr/vault.yml | grep '^samba_pass:' | sed 's/^samba_pass: *//' | tr -d '\"')"
  if smbclient "//debian.delarche.fr/data" -U "olivier%${SAMBA_PASS}" --max-protocol=SMB3 -c "put /dev/null test_write_must_fail" 2>&1; then
      echo "ERROR: Expected write to fail on read-only share, but it succeeded" >&2
      exit 1
  fi
  ```
- **Expected Result**:
  - `smbclient` fails with `NT_STATUS_ACCESS_DENIED` or `NT_STATUS_MEDIA_WRITE_PROTECTED`.

---

### `TC-SMB-06` — Role Idempotence (`system.yml --tags samba`)
- **Objective**: Verify that running `ansible/system.yml --tags samba` on a configured host results in zero changes (`changed=0`).
- **Preconditions**: `ansible/system.yml --tags samba` applied once.
- **Commands**:
  ```bash
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/system.yml --tags samba
  ```
- **Expected Result**:
  - Play recap shows `changed=0`, `failed=0`.

---

### `TC-SMB-07` — Minimum Protocol Enforcement (`server min protocol = SMB3`)
- **Objective**: Verify that legacy, insecure SMB dialects (such as SMB1 / `NT1`) are unconditionally rejected.
- **Preconditions**: `TC-SMB-01` passed.
- **Commands**:
  ```bash
  SAMBA_PASS="$(uv run ansible-vault view ansible/envs/lab/host_vars/debian.delarche.fr/vault.yml | grep '^samba_pass:' | sed 's/^samba_pass: *//' | tr -d '\"')"
  if smbclient "//debian.delarche.fr/data" -U "olivier%${SAMBA_PASS}" --max-protocol=NT1 -c "ls" 2>&1; then
      echo "ERROR: Expected NT1 connection to fail, but it succeeded" >&2
      exit 1
  fi
  ```
- **Expected Result**:
  - Connection is rejected (`NT_STATUS_NOT_SUPPORTED` or connection reset).

---

## 8. Advanced Corner-Case & Chaos Sequences (`TC-ADV`)

### `TC-ADV-01` — Post-Reboot Locked/Unmounted Pool 0ms Refusal (`Requisite=` + `PartOf=`)
- **Objective**: Simulate the daily `04:00` (`rclone`) and `05:00` (`snapshot`) systemd timers firing after an unattended reboot while LUKS/Btrfs is still locked/unmounted. Prove that `systemd` aborts both services in **0 ms** (`Dependency failed`) without hanging on `/dev/disk/by-label/data` or writing to the root `/` filesystem.
- **Failure Mode Prevented**: Filling up the root `/` partition by syncing cloud data into an unmounted `/home/olivier/data/sync` directory, or hanging systemd jobs for 90s waiting for a locked LUKS device.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "set -e
  sudo umount /home/olivier/data/sync
  ! sudo systemctl start rclone-sync-onedrive-share-50ans.service 2>&1
  ! sudo systemctl start btrfs-snapshot-data.service 2>&1
  test -z \"\$(ls -A /home/olivier/data/sync)\" && echo ROOT_MOUNTPOINT_CLEAN_OK
  sudo mount /home/olivier/data/sync
  "
  ```
- **Expected Result**:
  - Both `systemctl start` commands fail instantaneously (`Dependency failed for ...`, exit code `!= 0`).
  - `/home/olivier/data/sync` on the root filesystem remains completely empty (`ROOT_MOUNTPOINT_CLEAN_OK`).

---

### `TC-ADV-02` — Mid-Execution Unmount Kill-Switch (`PartOf=<unit>.mount`)
- **Objective**: Prove that if the underlying Btrfs mount unit (`home-olivier-data-sync.mount`) is stopped while a backup service is running, `systemd` (`PID 1`) immediately terminates the running service via `PartOf=`.
- **Failure Mode Prevented**: Zombie backup processes blocking storage maintenance or continuing to execute when storage is being stopped.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "set -e
  sudo mkdir -p /run/systemd/system/rclone-sync-onedrive-share-50ans.service.d
  printf '[Service]\nExecStartPre=/bin/sleep 15\n' | sudo tee /run/systemd/system/rclone-sync-onedrive-share-50ans.service.d/override.conf >/dev/null
  sudo systemctl daemon-reload
  sudo systemctl start --no-block rclone-sync-onedrive-share-50ans.service
  sleep 1
  systemctl show -p ActiveState,SubState rclone-sync-onedrive-share-50ans.service
  sudo systemctl stop home-olivier-data-sync.mount
  systemctl show -p ActiveState,SubState rclone-sync-onedrive-share-50ans.service
  sudo rm -rf /run/systemd/system/rclone-sync-onedrive-share-50ans.service.d
  sudo systemctl daemon-reload
  sudo mount /home/olivier/data/sync
  "
  ```
- **Expected Result**:
  - Before `systemctl stop home-olivier-data-sync.mount`: `ActiveState=activating`, `SubState=start-pre`.
  - Immediately after `systemctl stop home-olivier-data-sync.mount`: `ActiveState=failed` / `inactive` (terminated by PID 1).

---

### `TC-ADV-03` — Kernel Mount-Namespace Sandbox Probe (`ProtectSystem=strict` + `ReadWritePaths=`)
- **Objective**: Prove that the kernel mount namespace configured on `btrfs-snapshot-data.service` (`ProtectSystem=strict`, `ProtectHome=read-only`, `ReadWritePaths=/home/olivier/data/snapshots`) makes the entire OS (`/etc`), `/home`, and **even the source Btrfs subvolume (`/home/olivier/data/sync`) strictly read-only (`EROFS`)**, while allowing writes only to `/home/olivier/data/snapshots`.
- **Failure Mode Prevented**: A bug or compromised binary in the snapshot/backup job modifying or deleting source user data or system configuration files.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "sudo systemd-run --wait --pipe \
    -p NoNewPrivileges=true \
    -p PrivateTmp=true \
    -p ProtectSystem=strict \
    -p ProtectHome=read-only \
    -p ReadWritePaths=/home/olivier/data/snapshots \
    /bin/bash -c 'touch /home/olivier/data/sync/test_ro 2>&1 || true; touch /etc/test_ro 2>&1 || true; touch /home/olivier/data/snapshots/.rw_probe && rm -f /home/olivier/data/snapshots/.rw_probe && echo SNAPSHOTS_RW_OK'"
  ```
- **Expected Result**:
  - `touch: cannot touch '/home/olivier/data/sync/test_ro': Read-only file system`
  - `touch: cannot touch '/etc/test_ro': Read-only file system`
  - `SNAPSHOTS_RW_OK`

---

### `TC-ADV-04` — Snapshot Retention Pruning Boundaries (Daily / ISO-Weekly `%G-%V` / Expired)
- **Objective**: Verify the 3-tier retention algorithm in `snapshot.sh` (`KEEP_DAILY=30`, `KEEP_WEEKLY=52`):
  1. Snapshots `< 30` days old are all preserved.
  2. Snapshots between `30` and `394` days old keep **only the newest snapshot per ISO week (`%G-%V`)** and prune older duplicates in the same week.
  3. Snapshots `> 394` days old have their `ro` property cleared and are deleted.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "set -e
  sudo btrfs subvolume snapshot -r /home/olivier/data/sync /home/olivier/data/snapshots/2025-01-10--05-00-00
  sudo btrfs subvolume snapshot -r /home/olivier/data/sync /home/olivier/data/snapshots/2026-08-11--05-00-00
  sudo btrfs subvolume snapshot -r /home/olivier/data/sync /home/olivier/data/snapshots/2026-08-12--05-00-00
  sudo systemctl start btrfs-snapshot-data.service
  ls -1 /home/olivier/data/snapshots/
  test ! -d /home/olivier/data/snapshots/2025-01-10--05-00-00 && echo EXPIRED_PRUNED_OK
  test ! -d /home/olivier/data/snapshots/2026-08-11--05-00-00 && echo OLDER_ISO_WEEK_PRUNED_OK
  test -d /home/olivier/data/snapshots/2026-08-12--05-00-00 && echo NEWEST_ISO_WEEK_KEPT_OK
  sudo btrfs property set /home/olivier/data/snapshots/2026-08-12--05-00-00 ro false
  sudo btrfs subvolume delete /home/olivier/data/snapshots/2026-08-12--05-00-00
  "
  ```
- **Expected Result**:
  - `EXPIRED_PRUNED_OK`, `OLDER_ISO_WEEK_PRUNED_OK`, and `NEWEST_ISO_WEEK_KEPT_OK`.

---

### `TC-ADV-05` — Unformatted Disk Loss + Auto-Detected Degraded RAID1 Rebuild + `btrfs scrub`
- **Objective**: Simulate a complete physical disk failure and replacement with a blank unformatted drive (`wipefs -a` + `dd` zeroing on `backup-1`), then run `storage-replace.yml` **without** specifying `replace_pool` or `replace_index` so Ansible automatically detects the unformatted disk, mounts degraded, rebuilds the Btrfs RAID1 mirror, and passes a full `btrfs scrub`.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "set -e
  echo 'RAID1_DEGRADED_RECOVERY_OK' > /home/olivier/backup/sync/corner_case_test.txt && sync
  sudo umount /home/olivier/backup/sync /home/olivier/backup/snapshots
  sudo cryptsetup luksClose backup-1
  sudo wipefs -a /dev/disk/by-id/ata-debian-4_SSD_NWWFF9EGT4W4F6PE4PFK
  sudo dd if=/dev/zero of=/dev/disk/by-id/ata-debian-4_SSD_NWWFF9EGT4W4F6PE4PFK bs=1M count=16 status=none
  "
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-replace.yml -e confirm_replace=yes
  ssh olivier@debian.delarche.fr "cat /home/olivier/backup/sync/corner_case_test.txt && sudo btrfs scrub start -B /home/olivier/backup/sync && rm -f /home/olivier/backup/sync/corner_case_test.txt"
  ```
- **Expected Result**:
  - `storage-replace.yml` auto-detects `Pool: backup, Member Index: 1`, formats LUKS, runs `btrfs replace start` (`0 write errs, 0 uncorr. read errs`), and rebalances to RAID1.
  - `cat` outputs `RAID1_DEGRADED_RECOVERY_OK` and `btrfs scrub` reports `Error summary: no errors found`.

---

### `TC-ADV-06` — Corrupted LUKS2 Primary & Secondary Header Emergency Restore (`luksHeaderRestore`)
- **Objective**: Simulate severe corruption of both the primary (`0x0000`) and secondary (`0x4000`) LUKS2 headers and keyslots (`2 MiB` zeroed) on a member disk, confirm `cryptsetup isLuks` fails, restore the header from `/var/backups/luks/luks-header-backup-1.img`, unlock via `storage-unlock.yml`, and verify data integrity with `btrfs scrub`.
- **Commands**:
  ```bash
  ssh olivier@debian.delarche.fr "set -e
  echo 'HEADER_RESTORE_OK' > /home/olivier/backup/sync/header_test.txt && sync
  sudo umount /home/olivier/backup/sync /home/olivier/backup/snapshots
  sudo cryptsetup luksClose backup-1
  sudo dd if=/dev/zero of=/dev/disk/by-id/ata-debian-4_SSD_NWWFF9EGT4W4F6PE4PFK bs=1M count=2 conv=notrunc status=none
  ! sudo cryptsetup isLuks /dev/disk/by-id/ata-debian-4_SSD_NWWFF9EGT4W4F6PE4PFK && echo CORRUPTION_CONFIRMED
  sudo cryptsetup luksHeaderRestore /dev/disk/by-id/ata-debian-4_SSD_NWWFF9EGT4W4F6PE4PFK --header-backup-file /var/backups/luks/luks-header-backup-1.img --batch-mode
  sudo cryptsetup isLuks /dev/disk/by-id/ata-debian-4_SSD_NWWFF9EGT4W4F6PE4PFK && echo HEADER_RESTORED_OK
  "
  ANSIBLE_HOME=/tmp/.ansible uv run ansible-playbook -i ansible/envs/lab/inventory.ini ansible/storage-unlock.yml
  ssh olivier@debian.delarche.fr "cat /home/olivier/backup/sync/header_test.txt && sudo btrfs scrub start -B /home/olivier/backup/sync && rm -f /home/olivier/backup/sync/header_test.txt"
  ```
- **Expected Result**:
  - `CORRUPTION_CONFIRMED` followed by `HEADER_RESTORED_OK`.
  - `storage-unlock.yml` opens `backup-1` and mounts `/home/olivier/backup/sync` cleanly.
  - `cat` outputs `HEADER_RESTORE_OK` and `btrfs scrub` reports `no errors found`.

---

## 9. Master End-to-End Non-Regression Campaign Checklist

Use this ordered checklist to execute a complete validation campaign on `debian.delarche.fr`:

- [ ] **Step 1 — Static Quality Gates**: Run `TC-QA-01` (`pre-commit run --all-files`) and `TC-QA-02` (`--syntax-check`).
- [ ] **Step 2 — System Baseline & Hardening**: Run `TC-SYS-01` (`system.yml` twice, `changed=0` on 2nd run), `TC-SYS-02` (locales/keyboard), `TC-SYS-03` (NTP/leap seconds), and `TC-SYS-04` (multi-key SSH login).
- [ ] **Step 3 — Dual-Stack Firewall (`ufw`)**: Run `TC-FW-01` (netfilter rules), `TC-FW-02` (live port 9999 probe), and `TC-FW-03` (`ufw disable` remediation).
- [ ] **Step 4 — Unified Observability (`alloy`)**: Run `TC-OBS-01` (Alloy active state, cgroups sandboxing, and HTTP probe) and `TC-OBS-02` (LAN UFW rules & `status.yml --tags alloy`).
- [ ] **Step 5 — Storage Wipe, Unlock & Reboot Cycle**: Run `TC-STO-01` (`storage-wipe-and-reset.yml`), `TC-STO-02` (`storage-unlock.yml` idempotence), and `TC-STO-03` (full lock/unlock cycle).
- [ ] **Step 6 — Cloud Sync & Btrfs Snapshots (`systemd`)**: Run `TC-BAK-01` (`backup.yml` twice), `TC-BAK-02` (`OneDrive:/Share/50ans` sync), `TC-BAK-03` (`btrfs-snapshot-data.service`), `TC-BAK-04` (`status.yml`), `TC-BAK-05` (multi-destination sync & mount unit check), `TC-BAK-06` (negative validation suite), and `TC-BAK-07` (orphan unit cleanup).
- [ ] **Step 7 — Samba File Sharing & Shadow Copies**: Run `TC-SMB-01` (service health), `TC-SMB-02` (authenticated SMB3 listing), `TC-SMB-03` (shadow copies mapping), `TC-SMB-04` (read-only rejection), `TC-SMB-06` (`system.yml --tags samba` idempotence), and `TC-SMB-07` (SMB1 rejection).
- [ ] **Step 8 — Advanced Corner Cases & Chaos**: Run `TC-ADV-01` (unmounted 0ms refusal), `TC-ADV-02` (`PartOf=` unmount kill-switch), `TC-ADV-03` (EROFS sandbox probe), `TC-ADV-04` (ISO-week snapshot pruning), `TC-ADV-05` (wiped disk auto-detected RAID1 rebuild + scrub), and `TC-ADV-06` (LUKS2 header corruption & restore).

> [!NOTE]
> **Automated CI execution**: The full non-regression campaign (Steps 1–8) runs automatically on every commit across all branches via [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) on the self-hosted runner (`runner.delarche.fr`). To skip the destructive lab tests (e.g., for documentation-only changes), include `[skip-lab]` in the commit message. See [`docs/github-ci-runner-setup.md`](github-ci-runner-setup.md) for details.
