#! /bin/bash

SRC_DIR="/Users/olivier/"
DST_DIR="/Volumes/Mac Userdata Backup/backup/"

LOGFILE="/Users/olivier/OneDrive/Documents/geek/3. RUN/rsync/logs/external/$(date +%Y-%m-%d--%H-%M-%S)-rsync.log"

/usr/bin/rsync -av "$SRC_DIR" "$DST_DIR" --delete -F --log-file="$LOGFILE" --log-file-format="%o %i %n%L"
