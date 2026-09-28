#!/bin/bash
set -euo pipefail

usage() {
    local code="${1:-1}"
    cat <<EOF
Usage: $(basename "$0") [OPTIONS] <file1> [file2 ...]

Image conversion utility using ImageMagick (magick).

Options:
  --a4         Center image onto a white A4 canvas (300 DPI, 2480x3508)
  --alongside  Save converted output in the same directory as the input file
  --quality Q  JPEG compression quality (default: 70)
  --level L    Color level adjustment (default: 25%,75%)
  -h, --help   Display this help message
EOF
    exit "$code"
}

MODE_A4=false
MODE_ALONGSIDE=false
QUALITY="70"
LEVEL="25%,75%"
FILES=()

while [ "$#" -gt 0 ]; do
    case "$1" in
        --a4)
            MODE_A4=true
            shift
            ;;
        --alongside)
            MODE_ALONGSIDE=true
            shift
            ;;
        --quality)
            QUALITY="${2:?Missing quality value}"
            shift 2
            ;;
        --level)
            LEVEL="${2:?Missing level value}"
            shift 2
            ;;
        -h|--help)
            usage 0
            ;;
        --)
            shift
            while [ "$#" -gt 0 ]; do
                FILES+=("$1")
                shift
            done
            break
            ;;
        -*)
            echo "Error: Unknown option '$1'" >&2
            usage
            ;;
        *)
            FILES+=("$1")
            shift
            ;;
    esac
done

if [ "${#FILES[@]}" -eq 0 ]; then
    usage
fi

for infile in "${FILES[@]}"; do
    if [ ! -f "$infile" ]; then
        echo "Warning: File not found: '$infile'" >&2
        continue
    fi

    format="$(magick identify -format "%m" "$infile" 2>/dev/null || true)"
    if [[ "$format" != "PNG" && "$format" != "JPEG" && "$format" != "TIFF" ]]; then
        echo "Skipping '$infile' (unsupported format: ${format:-unknown})"
        continue
    fi

    filename="$(basename -- "$infile")"
    basename_no_ext="${filename%.*}"

    if [ "$MODE_ALONGSIDE" = true ]; then
        filedir="$(dirname "$infile")"
        outfile="$filedir/$basename_no_ext.jpg"
    else
        outfile="$basename_no_ext.jpg"
    fi

    if [ "$MODE_A4" = true ]; then
        echo "Converting '$infile' to A4 JPEG: '$outfile'"
        magick convert -size 2480x3508 xc:white "$infile" \
            -gravity center -composite \
            -level "$LEVEL" \
            -density 300 -page a4 -units PixelsPerInch \
            -quality "$QUALITY" "$outfile"
    else
        echo "Converting '$infile' to JPEG: '$outfile'"
        magick convert "$infile" \
            -level "$LEVEL" \
            -quality "$QUALITY" "$outfile"
    fi
done
