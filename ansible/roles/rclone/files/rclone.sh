#!/bin/bash
set -euo pipefail

SCRIPTNAME="$(basename "$0")"
# May be "onedrive" or "S3"
PROVIDER="${1:?Provider argument is required (e.g. onedrive, S3)}"
# Optional bucket name (blank for root remotes like onedrive)
BUCKET="${2:-}"

USER_HOME="${HOME:-/home/olivier}"
LOCKDIR="$USER_HOME/$SCRIPTNAME.$PROVIDER${BUCKET:+.$BUCKET}.lock"
CRYPTO_BASE_DIR="${CRYPTO_BASE_DIR:-$USER_HOME/crypto}"

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

    LOG_DIR="$CRYPTO_BASE_DIR/logs/$PROVIDER/$BUCKET"
    mkdir -p "$LOG_DIR"
    LOGFILE="$LOG_DIR/$(date +%Y-%m-%d--%H-%M-%S)-rclone.log"

    echo "Syncing $PROVIDER:$BUCKET to $CRYPTO_BASE_DIR/sync/$PROVIDER/$BUCKET"
    rclone -v --log-file="$LOGFILE" --stats-one-line sync "$PROVIDER:$BUCKET" "$CRYPTO_BASE_DIR/sync/$PROVIDER/$BUCKET"
    echo "Sync complete"

    # Prune logs older than 30 days
    echo "Pruning logs older than 30 days in $LOG_DIR"
    find "$LOG_DIR" -type f -name "*-rclone.log" -mtime +30 -delete
else
    echo "Could not create lock directory '$LOCKDIR' (another instance may be running)"
    exit 1
fi
