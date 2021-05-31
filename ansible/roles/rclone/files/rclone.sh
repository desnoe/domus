#!/bin/bash

SCRIPTNAME="$(basename $0)"
# may be "onedrive" or "S3"
PROVIDER="$1"
# leave blank for "onedrive"
BUCKET="$2"


LOCKDIR="/home/olivier/$SCRIPTNAME.$PROVIDER.$BUCKET.lock"

#Remove the lock directory
function cleanup {
    if rmdir $LOCKDIR; then
        echo "Finished"
    else
        echo "Failed to remove lock directory '$LOCKDIR'"
        exit 1
    fi
}

if mkdir $LOCKDIR; then
    #Ensure that if we "grabbed a lock", we release it
    trap "cleanup" EXIT SIGTERM SIGINT

    echo "Acquired lock, running"
    # Processing starts here

	LOGFILE="/home/olivier/crypto/logs/$PROVIDER/$BUCKET/$(date +%Y-%m-%d--%H-%M-%S)-rclone.log"
    mkdir -p $(dirname $LOGFILE)

	rclone -v --log-file="$LOGFILE" --stats-one-line sync "$PROVIDER:$BUCKET" "/home/olivier/crypto/sync/$PROVIDER/$BUCKET"

else
    echo "Could not create lock directory '$LOCKDIR'"
    exit 1
fi