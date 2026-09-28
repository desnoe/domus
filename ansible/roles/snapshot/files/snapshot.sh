#!/bin/bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
    echo "Usage: $0 <source_subvolume_path> <destination_snapshots_dir> [keep_daily] [keep_weekly]" >&2
    exit 1
fi

SOURCE_PATH="${1%/}"
DEST_PATH="${2%/}"
KEEP_DAILY="${3:-${SNAPSHOT_KEEP_DAILY:-30}}"
KEEP_WEEKLY="${4:-${SNAPSHOT_KEEP_WEEKLY:-52}}"

if ! [[ "$KEEP_DAILY" =~ ^[0-9]+$ ]] || ! [[ "$KEEP_WEEKLY" =~ ^[0-9]+$ ]]; then
    echo "Error: keep_daily and keep_weekly must be positive integers." >&2
    exit 1
fi

DATE="$(date +%Y-%m-%d--%H-%M-%S)"
echo "Creating Btrfs read-only snapshot: $DEST_PATH/$DATE"
btrfs subvolume snapshot -r "$SOURCE_PATH" "$DEST_PATH/$DATE"

prune_snapshots() {
    local now_epoch
    now_epoch="$(date +%s)"

    shopt -s nullglob
    local snap_dirs=("$DEST_PATH"/20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]--*)
    shopt -u nullglob

    if [ "${#snap_dirs[@]}" -eq 0 ]; then
        return 0
    fi

    local sorted_snaps=()
    local i
    for (( i=${#snap_dirs[@]}-1 ; i>=0 ; i-- )) ; do
        sorted_snaps+=( "${snap_dirs[i]}" )
    done

    local seen_weeks=" "
    local snap_dir snap_name snap_date snap_epoch age_days snap_week max_weekly_age
    max_weekly_age=$(( KEEP_DAILY + KEEP_WEEKLY * 7 ))

    for snap_dir in "${sorted_snaps[@]}"; do
        snap_name="$(basename "$snap_dir")"
        snap_date="${snap_name:0:10}"

        if ! snap_epoch="$(date -d "$snap_date" +%s 2>/dev/null)"; then
            continue
        fi

        age_days=$(( (now_epoch - snap_epoch) / 86400 ))

        # Case 1: Keep all snapshots younger than KEEP_DAILY days
        if [ "$age_days" -lt "$KEEP_DAILY" ]; then
            continue
        fi

        # Case 2: Keep newest snapshot for each distinct ISO week (%G-%V) up to max_weekly_age
        if [ "$age_days" -lt "$max_weekly_age" ]; then
            snap_week="$(date -d "$snap_date" +%G-%V 2>/dev/null || true)"
            if [ -n "$snap_week" ] && [[ "$seen_weeks" != *" $snap_week "* ]]; then
                seen_weeks+="${snap_week} "
                continue
            fi
        fi

        # Case 3: Expired or redundant weekly snapshot -> prune
        echo "Pruning expired snapshot: $snap_dir"
        btrfs property set "$snap_dir" ro false 2>/dev/null || true
        if ! btrfs subvolume delete "$snap_dir"; then
            echo "Warning: failed to delete snapshot $snap_dir" >&2
        fi
    done
}

echo "Rotating Btrfs snapshots in $DEST_PATH..."
prune_snapshots
echo "Snapshot rotation complete."
