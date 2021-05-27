#! /bin/bash

SRC_DIR=/Users/olivier/
DST_DIR=zotac.delarche.fr:/home/olivier/crypto/sync/mac/

LOGFILE="/Users/olivier/OneDrive/Geek/3. RUN/rsync/logs/$(date +%Y-%m-%d--%H-%M-%S)-rsync.log"

/usr/bin/rsync -av $SRC_DIR $DST_DIR --delete -F --log-file="$LOGFILE" --log-file-format="%o %i %n%L"
