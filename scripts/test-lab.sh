#!/usr/bin/env bash
# Lab non-regression integration test runner for homecode (targets debian.delarche.fr).
# Executed by GitHub Actions Job 2 (integration-tests-local) on runner.delarche.fr
# or manually from the controller via: ./scripts/test-lab.sh <subcommand|all>
#
# Security note: Secrets (~/.ansible_vault_pass and ~/.config/rclone/rclone.conf)
# persist on the self-hosted runner by design. The runner (runner.delarche.fr) is
# an on-prem machine owned and operated by the repository owner, and the repository
# is private. The check-prereqs step only writes these files if they are missing
# (initial provisioning via GitHub Secrets); once provisioned, files are reused
# across CI runs without re-creation.

set -euo pipefail

readonly LAB_INVENTORY="ansible/envs/lab/inventory.ini"
readonly LAB_HOST="${LAB_HOST:-debian.delarche.fr}"
readonly LAB_USER="${LAB_USER:-olivier}"

run_remote() {
    ssh -o BatchMode=yes -o ConnectTimeout=5 "${LAB_USER}@${LAB_HOST}" /bin/bash -euo pipefail
}

assert_playbook_idempotent() {
    local playbook="$1"
    shift
    local output

    output="$(uv run ansible-playbook -i "${LAB_INVENTORY}" "${playbook}" "$@")"
    printf '%s\n' "${output}"
    printf '%s\n' "${output}" | grep -q "changed=0"
}

assert_rclone_validation_fails() {
    local case_label="$1"
    local invalid_json="$2"
    local output
    local exit_code=0

    echo "Running TC-BAK-06 negative case: ${case_label}..."
    output="$(uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/backup.yml -e "${invalid_json}" 2>&1)" || exit_code=$?

    if [[ "${exit_code}" -eq 0 ]]; then
        echo "ERROR: Expected backup.yml to fail for case '${case_label}', but it succeeded." >&2
        printf '%s\n' "${output}" >&2
        return 1
    fi

    if ! printf '%s\n' "${output}" | grep -q "rclone_jobs validation failed:"; then
        echo "ERROR: Expected 'rclone_jobs validation failed:' in output for case '${case_label}'." >&2
        printf '%s\n' "${output}" >&2
        return 1
    fi
}

step_check_prereqs() {
    if [[ -n "${VAULT_SECRET:-}" ]] && [[ ! -s "${HOME}/.ansible_vault_pass" ]]; then
        printf '%s\n' "${VAULT_SECRET}" > "${HOME}/.ansible_vault_pass"
        chmod 600 "${HOME}/.ansible_vault_pass"
    fi

    if [[ -n "${RCLONE_CONF_SECRET:-}" ]] && [[ ! -s "${HOME}/.config/rclone/rclone.conf" ]]; then
        mkdir -p "${HOME}/.config/rclone"
        chmod 700 "${HOME}/.config/rclone"
        printf '%s\n' "${RCLONE_CONF_SECRET}" > "${HOME}/.config/rclone/rclone.conf"
        chmod 600 "${HOME}/.config/rclone/rclone.conf"
    fi

    test -s "${HOME}/.ansible_vault_pass" \
        || { echo "FATAL: ${HOME}/.ansible_vault_pass is missing or empty"; exit 1; }
    test -s "${HOME}/.config/rclone/rclone.conf" \
        || { echo "FATAL: ${HOME}/.config/rclone/rclone.conf is missing or empty"; exit 1; }

    run_remote << 'EOF'
sudo -n true
EOF
}

step_system_and_ufw() {
    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/system.yml
    assert_playbook_idempotent ansible/system.yml

    run_remote << 'EOF'
sudo ufw disable
EOF

    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/system.yml

    run_remote << 'EOF'
sudo ufw status verbose | grep -q "Status: active"
EOF
}

step_storage_wipe_and_unlock() {
    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/storage-wipe-and-reset.yml -e confirm_wipe=yes
    assert_playbook_idempotent ansible/storage-unlock.yml
}

step_backup_and_sync() {
    # TC-BAK-01: Deployment & Idempotence
    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/backup.yml
    assert_playbook_idempotent ansible/backup.yml

    # TC-BAK-02 & TC-BAK-03: Primary pool sync & snapshot
    run_remote << 'EOF'
sudo systemctl start rclone-sync-onedrive-share-50ans.service
count="$(rclone ls /home/olivier/data/sync/OneDrive/Share/50ans | wc -l | tr -d ' ')"
test "${count}" -eq 8
sudo systemctl start btrfs-snapshot-data.service
EOF

    # TC-BAK-05: Multi-destination sync, dynamic systemd-escape mount unit resolution & secondary snapshot
    run_remote << 'EOF'
sudo systemctl start rclone-sync-onedrive-share-50ans-backup.service
count="$(rclone ls /home/olivier/backup/sync/onedrive/50ans | wc -l | tr -d ' ')"
test "${count}" -eq 8
req_unit="$(systemctl show -p Requisite --value rclone-sync-onedrive-share-50ans-backup.service)"
test "${req_unit}" = "home-olivier-backup-sync.mount"
systemctl is-active "${req_unit}" | grep -q "active"

sudo systemctl start btrfs-snapshot-backup.service
snap_dir="$(ls -td /home/olivier/backup/snapshots/20* | head -n 1)"
sudo btrfs property get "${snap_dir}" ro | grep -q "ro=true"
rclone ls "${snap_dir}/onedrive/50ans" | wc -l | tr -d ' ' | grep -q '^8$'
EOF

    # TC-BAK-06: Negative validation suite
    assert_rclone_validation_fails "duplicate-name" \
        '{"rclone_jobs": [{"name": "dup", "source": {"provider": "OneDrive"}, "destination": {"mount": "/home/olivier/data/sync", "path": "p1"}}, {"name": "dup", "source": {"provider": "OneDrive"}, "destination": {"mount": "/home/olivier/data/sync", "path": "p2"}}]}'
    assert_rclone_validation_fails "overlapping-targets" \
        '{"rclone_jobs": [{"name": "j1", "source": {"provider": "OneDrive"}, "destination": {"mount": "/home/olivier/data/sync", "path": "parent"}}, {"name": "j2", "source": {"provider": "OneDrive"}, "destination": {"mount": "/home/olivier/data/sync", "path": "parent/child"}}]}'
    assert_rclone_validation_fails "absolute-dest-path" \
        '{"rclone_jobs": [{"name": "j1", "source": {"provider": "OneDrive"}, "destination": {"mount": "/home/olivier/data/sync", "path": "/absolute"}}]}'
    assert_rclone_validation_fails "traversal-dest-path" \
        '{"rclone_jobs": [{"name": "j1", "source": {"provider": "OneDrive"}, "destination": {"mount": "/home/olivier/data/sync", "path": "../etc"}}]}'
    assert_rclone_validation_fails "invalid-regex-name" \
        '{"rclone_jobs": [{"name": "Bad_Name", "source": {"provider": "OneDrive"}, "destination": {"mount": "/home/olivier/data/sync", "path": "p1"}}]}'
    assert_rclone_validation_fails "unknown-root-keys" \
        '{"rclone_jobs": [{"name": "j1", "unknown_key": "val", "source": {"provider": "OneDrive"}, "destination": {"mount": "/home/olivier/data/sync", "path": "p1"}}]}'
    assert_rclone_validation_fails "legacy-flat-schema" \
        '{"rclone_jobs": [{"provider": "OneDrive", "bucket": "Share/50ans", "destination": "/home/olivier/data/sync"}]}'
    assert_rclone_validation_fails "undeclared-subvolume" \
        '{"rclone_jobs": [{"name": "j1", "source": {"provider": "OneDrive"}, "destination": {"mount": "/tmp", "path": "p1"}}]}'

    # TC-BAK-07: Automatic cleanup of orphan units
    run_remote << 'EOF'
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
EOF

    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/backup.yml

    run_remote << 'EOF'
test ! -f /etc/systemd/system/rclone-sync-orphan-test.service
test ! -f /etc/systemd/system/rclone-sync-orphan-test.timer
! systemctl is-active rclone-sync-orphan-test.timer >/dev/null 2>&1
systemctl is-active rclone-sync-onedrive-share-50ans.timer | grep -q "active"
EOF
}

step_samba() {
    if ! command -v smbclient &>/dev/null; then
        echo "Installing smbclient on controller..."
        if command -v apt-get &>/dev/null; then
            sudo apt-get update -qq && sudo apt-get install -y -qq smbclient
        elif command -v brew &>/dev/null; then
            brew install samba
        else
            echo "FATAL: smbclient not found and cannot be installed automatically" >&2
            exit 1
        fi
    fi

    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/system.yml --tags samba
    assert_playbook_idempotent ansible/system.yml --tags samba

    # TC-SMB-01: Service active & listening on 445/tcp
    run_remote << 'EOF'
systemctl is-active smbd
ss -tlnp sport = :445 | grep -q ':445'
EOF

    local samba_pass
    samba_pass="$(uv run ansible-vault view \
        ansible/envs/lab/host_vars/debian.delarche.fr/vault.yml \
        | grep '^samba_pass:' | sed 's/^samba_pass: *//' | tr -d '"')"

    # TC-SMB-02: Connect and list share files (data and backup shares)
    smbclient "//${LAB_HOST}/data" \
        -U "${LAB_USER}%${samba_pass}" \
        --max-protocol=SMB3 \
        -c "ls" | grep -q '\.'

    smbclient "//${LAB_HOST}/backup" \
        -U "${LAB_USER}%${samba_pass}" \
        --max-protocol=SMB3 \
        -c "ls" | grep -q '\.'

    # TC-SMB-03: Shadow copies configuration & snapshot verification
    run_remote << 'EOF'
testparm -s 2>/dev/null | grep -q 'vfs objects = shadow_copy2'
testparm -s 2>/dev/null | grep -q 'shadow:format = %Y-%m-%d--%H-%M-%S'
count="$(find /home/olivier/data/snapshots -maxdepth 1 -mindepth 1 -type d | wc -l | tr -d ' ')"
test "${count}" -ge 1
EOF

    # TC-SMB-04: Read-only enforcement (file upload must be rejected)
    if smbclient "//${LAB_HOST}/data" \
        -U "${LAB_USER}%${samba_pass}" \
        --max-protocol=SMB3 \
        -c "put /dev/null test_write_must_fail" 2>&1; then
        echo "ERROR: Expected write to fail on read-only data share, but it succeeded" >&2
        exit 1
    fi

    if smbclient "//${LAB_HOST}/backup" \
        -U "${LAB_USER}%${samba_pass}" \
        --max-protocol=SMB3 \
        -c "put /dev/null test_write_must_fail" 2>&1; then
        echo "ERROR: Expected write to fail on read-only backup share, but it succeeded" >&2
        exit 1
    fi

    # TC-SMB-07: Minimum SMB3 protocol enforcement (NT1 / SMB1 must be rejected)
    if smbclient "//${LAB_HOST}/data" \
        -U "${LAB_USER}%${samba_pass}" \
        --max-protocol=NT1 \
        -c "ls" 2>&1; then
        echo "ERROR: Expected NT1 connection to fail, but it succeeded" >&2
        exit 1
    fi
}

step_corner_cases_and_raid1() {
    run_remote << 'EOF'
readonly BACKUP_DISK_1="/dev/disk/by-id/ata-VBOX_HARDDISK_VBb957bdc8-b76337e3"

# TC-ADV-01: Unmounted 0ms refusal (Requisite= + AssertPathIsMountPoint=)
sudo umount /home/olivier/data/sync
! sudo systemctl start rclone-sync-onedrive-share-50ans.service 2>&1
! sudo systemctl start btrfs-snapshot-data.service 2>&1
sudo mount /home/olivier/data/sync

sudo umount /home/olivier/backup/sync
! sudo systemctl start rclone-sync-onedrive-share-50ans-backup.service 2>&1
! sudo systemctl start btrfs-snapshot-backup.service 2>&1
sudo mount /home/olivier/backup/sync

# TC-ADV-03: Kernel mount-namespace EROFS sandbox probe
sudo systemd-run --wait --pipe \
    -p NoNewPrivileges=true \
    -p PrivateTmp=true \
    -p ProtectSystem=strict \
    -p ProtectHome=read-only \
    -p ReadWritePaths=/home/olivier/data/snapshots \
    /bin/bash -c '! touch /home/olivier/data/sync/test_ro 2>/dev/null && ! touch /etc/test_ro 2>/dev/null && touch /home/olivier/data/snapshots/.rw_probe && rm -f /home/olivier/data/snapshots/.rw_probe'

# TC-ADV-06: LUKS2 header corruption & restore from /var/backups/luks/
echo "CI_SURVIVAL_OK" > /home/olivier/backup/sync/ci_test.txt
sync
sudo umount /home/olivier/backup/sync /home/olivier/backup/snapshots
sudo cryptsetup luksClose backup-1
sudo dd if=/dev/zero of="${BACKUP_DISK_1}" bs=1M count=2 conv=notrunc status=none
! sudo cryptsetup isLuks "${BACKUP_DISK_1}"
sudo cryptsetup luksHeaderRestore "${BACKUP_DISK_1}" \
    --header-backup-file /var/backups/luks/luks-header-backup-1.img \
    --batch-mode
EOF

    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/storage-unlock.yml

    run_remote << 'EOF'
readonly BACKUP_DISK_1="/dev/disk/by-id/ata-VBOX_HARDDISK_VBb957bdc8-b76337e3"

# TC-ADV-05 / TC-STO-04: Simulate complete disk replacement (wipefs + zero first 16 MiB)
sudo umount /home/olivier/backup/sync /home/olivier/backup/snapshots
sudo cryptsetup luksClose backup-1
sudo wipefs -a "${BACKUP_DISK_1}"
sudo dd if=/dev/zero of="${BACKUP_DISK_1}" bs=1M count=16 status=none
EOF

    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/storage-replace.yml -e confirm_replace=yes

    run_remote << 'EOF'
grep -q "CI_SURVIVAL_OK" /home/olivier/backup/sync/ci_test.txt
sudo btrfs scrub start -B /home/olivier/backup/sync
rm -f /home/olivier/backup/sync/ci_test.txt
EOF

    # TC-STO-05: Non-interactive pool removal via storage-remove.yml
    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/storage-remove.yml \
        -e remove_pool=backup -e confirm_remove=yes

    run_remote << 'EOF'
readonly BACKUP_DISK_0="/dev/disk/by-id/ata-VBOX_HARDDISK_VB3a4d115e-5d14d18f"
readonly BACKUP_DISK_1="/dev/disk/by-id/ata-VBOX_HARDDISK_VBb957bdc8-b76337e3"

! mountpoint -q /home/olivier/backup/sync
[ ! -e /dev/mapper/backup-0 ]
[ ! -e /dev/mapper/backup-1 ]
! sudo cryptsetup isLuks "${BACKUP_DISK_0}"
! sudo cryptsetup isLuks "${BACKUP_DISK_1}"
! grep -q 'LABEL=backup' /etc/fstab
! sudo test -f /var/backups/luks/luks-header-backup-0.img
! sudo test -f /var/backups/luks/luks-header-backup-1.img
EOF

    # Restore backup pool so lab remains fully provisioned
    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/storage-wipe-and-reset.yml \
        -e confirm_wipe=yes

    # TC-STO-06: Member disk removal & live index compaction via storage-disk-remove.yml
    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/storage-disk-remove.yml \
        -e pool=backup -e disk_index=0 -e confirm_remove=yes

    run_remote << 'EOF'
readonly BACKUP_DISK_0="/dev/disk/by-id/ata-VBOX_HARDDISK_VB3a4d115e-5d14d18f"

sudo btrfs filesystem df /home/olivier/backup/sync | grep -q 'Data, single'
[ -e /dev/mapper/backup-0 ]
[ ! -e /dev/mapper/backup-1 ]
! sudo cryptsetup isLuks "${BACKUP_DISK_0}"
sudo test -f /var/backups/luks/luks-header-backup-0.img
! sudo test -f /var/backups/luks/luks-header-backup-1.img
EOF

    # TC-STO-07: Member disk addition & RAID1 conversion via storage-disk-add.yml
    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/storage-disk-add.yml \
        -e pool=backup -e device=/dev/disk/by-id/ata-VBOX_HARDDISK_VB3a4d115e-5d14d18f -e confirm_add=yes

    run_remote << 'EOF'
readonly BACKUP_DISK_0="/dev/disk/by-id/ata-VBOX_HARDDISK_VB3a4d115e-5d14d18f"
readonly BACKUP_DISK_1="/dev/disk/by-id/ata-VBOX_HARDDISK_VBb957bdc8-b76337e3"

sudo btrfs filesystem df /home/olivier/backup/sync | grep -q 'Data, RAID1'
[ -e /dev/mapper/backup-0 ]
[ -e /dev/mapper/backup-1 ]
sudo cryptsetup isLuks "${BACKUP_DISK_0}"
sudo cryptsetup isLuks "${BACKUP_DISK_1}"
sudo test -f /var/backups/luks/luks-header-backup-0.img
sudo test -f /var/backups/luks/luks-header-backup-1.img
sudo btrfs scrub start -B /home/olivier/backup/sync
EOF
}

step_status() {
    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/status.yml
}

usage() {
    cat << 'EOF'
Usage: ./scripts/test-lab.sh <subcommand>

Subcommands:
  check-prereqs            Verify Vault password, rclone.conf, and SSH/sudo access to debian.delarche.fr
  system-and-ufw           Verify system.yml idempotence and UFW drift remediation (TC-SYS-01, TC-FW-03)
  storage-wipe-and-unlock  Run storage-wipe-and-reset.yml and verify storage-unlock.yml idempotence (TC-STO-01..02)
  backup-and-sync          Verify backup.yml idempotence, OneDrive sync (primary & secondary), snapshots, TC-BAK-05..07
  samba                    Deploy Samba, verify idempotence, SMB connectivity, shadow copies, and security (TC-SMB-01..07)
  corner-cases-and-raid1   Run TC-ADV-01..06, RAID1 disk replacement, pool removal, disk add/remove (TC-STO-04..07)
  status                   Run full read-only health report via ansible/status.yml (TC-BAK-04)
  all                      Execute the complete non-regression campaign sequentially
EOF
}

main() {
    local subcommand="${1:-}"

    case "${subcommand}" in
        check-prereqs)
            step_check_prereqs
            ;;
        system-and-ufw)
            step_system_and_ufw
            ;;
        storage-wipe-and-unlock)
            step_storage_wipe_and_unlock
            ;;
        backup-and-sync)
            step_backup_and_sync
            ;;
        samba)
            step_samba
            ;;
        corner-cases-and-raid1)
            step_corner_cases_and_raid1
            ;;
        status)
            step_status
            ;;
        all)
            step_check_prereqs
            step_system_and_ufw
            step_storage_wipe_and_unlock
            step_backup_and_sync
            step_samba
            step_corner_cases_and_raid1
            step_status
            ;;
        *)
            usage >&2
            exit 1
            ;;
    esac
}

main "$@"
