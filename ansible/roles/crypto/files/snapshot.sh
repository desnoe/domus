#!/bin/bash

DATE=`/bin/date +%Y-%m-%d--%H-%M-%S`
[ -e '/bin/btrfs' ] && BTRFS=/bin/btrfs
[ -e '/sbin/btrfs' ] && BTRFS=/sbin/btrfs

SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
cd $SCRIPTPATH

$BTRFS subvolume snapshot -r sync/ snapshots/$DATE
