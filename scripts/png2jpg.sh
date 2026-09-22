#!/bin/bash
set -euo pipefail

for infile in "$@"
do
    format=$(magick identify -format "%m" "$infile" 2>/dev/null || true)
    if [[ "$format" == "PNG" ]]; then
        outfile="$(basename "$infile" .png).jpg"
        echo magick convert "$infile" -level 25%,75% -quality 70 "$outfile"
        magick convert "$infile" -level 25%,75% -quality 70 "$outfile"
    fi
done
