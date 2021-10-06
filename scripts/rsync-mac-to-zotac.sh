#! /bin/bash

SRC_DIR="/Users/olivier/"
DST_DIR="zotac.delarche.fr:/home/olivier/crypto/sync/mac/"

LOGFILE="/Users/olivier/OneDrive/Documents/geek/3. RUN/rsync/logs/zotac/$(date +%Y-%m-%d--%H-%M-%S)-rsync.log"

/usr/bin/rsync -av "$SRC_DIR" "$DST_DIR" --delete -F --filter='dir-merge /.rsync-cloud-filter' --log-file="$LOGFILE" --log-file-format="%o %i %n%L"
