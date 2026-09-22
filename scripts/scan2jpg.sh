#!/bin/bash
set -euo pipefail

for infile in "$@"
do
    absfilename=$(realpath -s "$infile")
    absfiledir=$(dirname "$absfilename")

    filename=$(basename -- "$infile")
    filename="${filename%.*}"

    format=$(magick identify -format "%m" "$infile" 2>/dev/null || true)
    if [[ "$format" == "PNG" || "$format" == "JPEG" ]]; then
        outfile="$absfiledir/$filename.jpg"
        echo magick convert "$infile" -level 25%,75% -quality 70 "$outfile"
        magick convert "$infile" -level 25%,75% -quality 70 "$outfile"
    fi
done
