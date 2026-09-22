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

"$BTRFS" subvolume snapshot -r sync/ "snapshots/$DATE"
