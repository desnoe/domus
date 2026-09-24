#!/bin/bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
    echo "Usage: $0 <source_subvolume_path> <destination_snapshots_dir> [keep_daily] [keep_weekly]" >&2
    exit 1
fi

SOURCE_RAW="$1"
DEST_RAW="$2"
KEEP_DAILY="${3:-${SNAPSHOT_KEEP_DAILY:-30}}"
KEEP_WEEKLY="${4:-${SNAPSHOT_KEEP_WEEKLY:-52}}"

if ! [[ "$KEEP_DAILY" =~ ^[0-9]+$ ]] || ! [[ "$KEEP_WEEKLY" =~ ^[0-9]+$ ]]; then
    echo "Error: keep_daily and keep_weekly must be positive integers." >&2
    exit 1
fi

BTRFS="btrfs"
if [ -e '/bin/btrfs' ]; then
    BTRFS=/bin/btrfs
elif [ -e '/sbin/btrfs' ]; then
    BTRFS=/sbin/btrfs
fi

# Resolve paths
if [ ! -d "$SOURCE_RAW" ]; then
    echo "Error: Source directory '$SOURCE_RAW' does not exist." >&2
    exit 1
fi
SOURCE_PATH="$(cd "$SOURCE_RAW" && pwd -P)"

if [ ! -d "$DEST_RAW" ]; then
    echo "Error: Destination directory '$DEST_RAW' does not exist." >&2
    exit 1
fi
DEST_PATH="$(cd "$DEST_RAW" && pwd -P)"

# Validate filesystem types
SOURCE_FS="$(stat -f -c %T "$SOURCE_PATH" 2>/dev/null || true)"
if [ "$SOURCE_FS" != "btrfs" ]; then
    echo "Error: Source path '$SOURCE_PATH' is not on a Btrfs filesystem (detected: '$SOURCE_FS')." >&2
    exit 1
fi

DEST_FS="$(stat -f -c %T "$DEST_PATH" 2>/dev/null || true)"
if [ "$DEST_FS" != "btrfs" ]; then
    echo "Error: Destination path '$DEST_PATH' is not on a Btrfs filesystem (detected: '$DEST_FS')." >&2
    exit 1
fi

# Validate that source is a Btrfs subvolume (inode 256 or subvolume show)
SOURCE_INODE="$(stat -c %i "$SOURCE_PATH" 2>/dev/null || true)"
if [ "$SOURCE_INODE" != "256" ] && ! "$BTRFS" subvolume show "$SOURCE_PATH" >/dev/null 2>&1; then
    echo "Error: Source path '$SOURCE_PATH' is not a Btrfs subvolume." >&2
    exit 1
fi

# Ensure single execution per destination with non-blocking file lock
if command -v md5sum >/dev/null 2>&1; then
    DEST_HASH="$(echo -n "$DEST_PATH" | md5sum | awk '{print $1}')"
elif command -v md5 >/dev/null 2>&1; then
    DEST_HASH="$(echo -n "$DEST_PATH" | md5)"
else
    DEST_HASH="$(echo -n "$DEST_PATH" | tr '/ ' '__')"
fi

LOCKFILE="/tmp/btrfs-snapshot-${DEST_HASH}.lock"
exec 200>"$LOCKFILE"
if ! flock -n 200; then
    echo "Another snapshot process is currently active for destination '$DEST_PATH'. Exiting." >&2
    exit 1
fi

DATE="$(/bin/date +%Y-%m-%d--%H-%M-%S)"
echo "Creating Btrfs snapshot: $DEST_PATH/$DATE"
"$BTRFS" subvolume snapshot -r "$SOURCE_PATH" "$DEST_PATH/$DATE"

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
    local now_epoch
    now_epoch="$(date +%s)"

    # Look for snapshots matching format: YYYY-MM-DD--HH-MM-SS
    shopt -s nullglob
    local snap_dirs=("$DEST_PATH"/20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]--*)
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
    max_weekly_age=$(( KEEP_DAILY + KEEP_WEEKLY * 7 ))

    for snap_dir in "${sorted_snaps[@]}"; do
        snap_name="$(basename "$snap_dir")"
        snap_date="${snap_name:0:10}"

        if ! snap_epoch="$(parse_epoch "$snap_date" 2>/dev/null)"; then
            continue
        fi

        age_days=$(( (now_epoch - snap_epoch) / 86400 ))

        # Case 1: Keep all snapshots younger than KEEP_DAILY days
        if [ "$age_days" -lt "$KEEP_DAILY" ]; then
            continue
        fi

        # Case 2: For snapshots between KEEP_DAILY and max_weekly_age,
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
        "$BTRFS" property set "$snap_dir" ro false 2>/dev/null || true
        if ! "$BTRFS" subvolume delete "$snap_dir"; then
            echo "Warning: failed to delete snapshot $snap_dir" >&2
        fi
    done
}

echo "Rotating Btrfs snapshots in $DEST_PATH..."
prune_snapshots
echo "Snapshot rotation complete."
