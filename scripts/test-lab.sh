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
    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/backup.yml
    assert_playbook_idempotent ansible/backup.yml

    run_remote << 'EOF'
sudo systemctl start rclone-sync-onedrive-share-50ans.service
count="$(rclone ls /home/olivier/data/sync/OneDrive/Share/50ans | wc -l | tr -d ' ')"
test "${count}" -eq 8
sudo systemctl start btrfs-snapshot-data.service
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

    # TC-SMB-02: Connect and list share files
    smbclient "//${LAB_HOST}/data" \
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
        echo "ERROR: Expected write to fail on read-only share, but it succeeded" >&2
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
readonly BACKUP_DISK_1="/dev/disk/by-id/ata-debian-4_SSD_NWWFF9EGT4W4F6PE4PFK"

# TC-ADV-01: Unmounted 0ms refusal (Requisite= + AssertPathIsMountPoint=)
sudo umount /home/olivier/data/sync
! sudo systemctl start rclone-sync-onedrive-share-50ans.service 2>&1
! sudo systemctl start btrfs-snapshot-data.service 2>&1
sudo mount /home/olivier/data/sync

# TC-ADV-03: Kernel mount-namespace EROFS sandbox probe
sudo systemd-run --wait --pipe \
    -p NoNewPrivileges=true \
    -p PrivateTmp=true \
    -p ProtectSystem=strict \
    -p ProtectHome=read-only \
    -p ReadWritePaths=/home/olivier/data/snapshots \
    /bin/bash -c '! touch /home/olivier/data/sync/test_ro 2>/dev/null && ! touch /etc/test_ro 2>/dev/null && touch /home/olivier/data/snapshots/.rw_probe && rm -f /home/olivier/data/snapshots/.rw_probe'

# TC-ADV-06: LUKS2 header corruption & restore from /var/backups/luks/
echo "CI_SURVIVAL_OK" > /home/olivier/backup/ci_test.txt
sync
sudo umount /home/olivier/backup
sudo cryptsetup luksClose backup-1
sudo dd if=/dev/zero of="${BACKUP_DISK_1}" bs=1M count=2 conv=notrunc status=none
! sudo cryptsetup isLuks "${BACKUP_DISK_1}"
sudo cryptsetup luksHeaderRestore "${BACKUP_DISK_1}" \
    --header-backup-file /var/backups/luks/luks-header-backup-1.img \
    --batch-mode
EOF

    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/storage-unlock.yml

    run_remote << 'EOF'
readonly BACKUP_DISK_1="/dev/disk/by-id/ata-debian-4_SSD_NWWFF9EGT4W4F6PE4PFK"

# TC-ADV-05 / TC-STO-04: Simulate complete disk replacement (wipefs + zero first 16 MiB)
sudo umount /home/olivier/backup
sudo cryptsetup luksClose backup-1
sudo wipefs -a "${BACKUP_DISK_1}"
sudo dd if=/dev/zero of="${BACKUP_DISK_1}" bs=1M count=16 status=none
EOF

    uv run ansible-playbook -i "${LAB_INVENTORY}" ansible/storage-replace.yml -e confirm_replace=yes

    run_remote << 'EOF'
grep -q "CI_SURVIVAL_OK" /home/olivier/backup/ci_test.txt
sudo btrfs scrub start -B /home/olivier/backup
rm -f /home/olivier/backup/ci_test.txt
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
  backup-and-sync          Verify backup.yml idempotence, OneDrive:/Share/50ans sync, and Btrfs snapshot (TC-BAK-01..03)
  samba                    Deploy Samba, verify idempotence, SMB connectivity, shadow copies, and security (TC-SMB-01..07)
  corner-cases-and-raid1   Run TC-ADV-01..06 corner cases, LUKS header restore, and RAID1 disk replacement
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
