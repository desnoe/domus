#! /bin/bash

for infile in "$@"
do
	absfilename=$(realpath -s "$infile")
	absfiledir=$(dirname "$absfilename")

	filename=$(basename -- "$infile")
	filename="${filename%.*}"
	
    id=$(magick identify "$infile")
    format=$(echo ${id:${#infile}} | cut -d' ' -f1)
    if [[ "$format" == "PNG" || "$format" == "JPEG" ]]; then
        outfile="$absfiledir/$filename.jpg"
        echo magick convert "$infile" -level 25%,75%  -quality 70 "$outfile"
        magick convert "$infile" -level 25%,75%  -quality 70 "$outfile"
    fi
done
