#!/bin/bash
set -euo pipefail

SCRIPTNAME="$(basename "$0")"
# May be "onedrive" or "S3"
PROVIDER="${1:?Provider argument is required (e.g. onedrive, S3)}"
# Target synchronization base directory (e.g. /home/olivier/data/sync)
DESTINATION="${2:?Destination directory argument is required}"
# Optional bucket name (blank for root remotes like onedrive)
BUCKET="${3:-}"

USER_HOME="${HOME:-/home/$(id -un)}"
LOCKDIR="$USER_HOME/$SCRIPTNAME.$PROVIDER${BUCKET:+.$BUCKET}.lock"

# Remove the lock directory on exit
cleanup() {
    if [ -d "$LOCKDIR" ]; then
        rmdir "$LOCKDIR" 2>/dev/null || rm -rf "$LOCKDIR"
        echo "Finished and released lock '$LOCKDIR'"
    fi
}

if mkdir "$LOCKDIR" 2>/dev/null; then
    trap "cleanup" EXIT INT TERM

    echo "Acquired lock, starting sync process"

    LOG_DIR="$USER_HOME/logs/$PROVIDER${BUCKET:+/$BUCKET}"
    mkdir -p "$LOG_DIR"
    LOGFILE="$LOG_DIR/$(date +%Y-%m-%d--%H-%M-%S)-rclone.log"

    TARGET_DIR="$DESTINATION/$PROVIDER${BUCKET:+/$BUCKET}"
    echo "Syncing $PROVIDER:$BUCKET to $TARGET_DIR"
    rclone -v --log-file="$LOGFILE" --stats-one-line sync "$PROVIDER:$BUCKET" "$TARGET_DIR"
    echo "Sync complete"

    # Prune logs older than 30 days
    echo "Pruning logs older than 30 days in $LOG_DIR"
    find "$LOG_DIR" -type f -name "*-rclone.log" -mtime +30 -delete
else
    echo "Could not create lock directory '$LOCKDIR' (another instance may be running)"
    exit 1
fi
