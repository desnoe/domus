#!/bin/bash
set -euo pipefail

DATE="$(/bin/date +%Y-%m-%d--%H-%M-%S)"
BTRFS="btrfs"
if [ -e '/bin/btrfs' ]; then
    BTRFS=/bin/btrfs
elif [ -e '/sbin/btrfs' ]; then
    BTRFS=/sbin/btrfs
fi

SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 && pwd -P )"
cd "$SCRIPTPATH"

echo "Creating Btrfs snapshot: snapshots/$DATE"
"$BTRFS" subvolume snapshot -r sync/ "snapshots/$DATE"

parse_epoch() {
    local d="$1"
    if date --version >/dev/null 2>&1; then
        date -d "$d" +%s
    else
        date -j -f "%Y-%m-%d" "$d" +%s
    fi
}

get_iso_week() {
    local d="$1"
    if date --version >/dev/null 2>&1; then
        date -d "$d" +%G-%V
    else
        date -j -f "%Y-%m-%d" "$d" +%G-%V
    fi
}

prune_snapshots() {
    local keep_daily=7
    local keep_weekly=4
    local now_epoch
    now_epoch="$(date +%s)"

    # Look for snapshots matching format: YYYY-MM-DD--HH-MM-SS
    shopt -s nullglob
    local snap_dirs=(snapshots/20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]--*)
    shopt -u nullglob

    if [ "${#snap_dirs[@]}" -eq 0 ]; then
        return 0
    fi

    # Sort descending (newest first)
    local sorted_snaps=()
    local i
    for (( i=${#snap_dirs[@]}-1 ; i>=0 ; i-- )) ; do
        sorted_snaps+=( "${snap_dirs[i]}" )
    done

    local seen_weeks=" "

    local snap_dir snap_name snap_date snap_epoch age_days snap_week max_weekly_age
    max_weekly_age=$(( keep_daily + keep_weekly * 7 ))

    for snap_dir in "${sorted_snaps[@]}"; do
        snap_name="$(basename "$snap_dir")"
        snap_date="${snap_name:0:10}"

        if ! snap_epoch="$(parse_epoch "$snap_date" 2>/dev/null)"; then
            continue
        fi

        age_days=$(( (now_epoch - snap_epoch) / 86400 ))

        # Case 1: Keep all snapshots younger than keep_daily days
        if [ "$age_days" -lt "$keep_daily" ]; then
            continue
        fi

        # Case 2: For snapshots between keep_daily and max_weekly_age,
        # keep newest snapshot for each distinct ISO week (%G-%V)
        if [ "$age_days" -lt "$max_weekly_age" ]; then
            snap_week="$(get_iso_week "$snap_date" 2>/dev/null || true)"
            if [ -n "$snap_week" ] && [[ "$seen_weeks" != *" $snap_week "* ]]; then
                seen_weeks+="${snap_week} "
                continue
            fi
        fi

        # Case 3: Expired or redundant weekly snapshot -> prune
        echo "Pruning expired snapshot: $snap_dir"
        if ! "$BTRFS" subvolume delete "$snap_dir"; then
            echo "Warning: failed to delete snapshot $snap_dir" >&2
        fi
    done
}

echo "Rotating Btrfs snapshots..."
prune_snapshots
echo "Snapshot rotation complete."
