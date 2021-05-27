#! /bin/bash

for infile in "$@"
do
    id=$(magick identify "$infile")
    format=$(echo ${id:${#infile}} | cut -d' ' -f1)
    if [[ "$format" == "PNG" ]]; then
        outfile="$(basename "$infile" .png).jpg"
        echo magick convert "$infile" -level 25%,75%  -quality 70 "$outfile"
        magick convert "$infile" -level 25%,75%  -quality 70 "$outfile"
    fi
done
