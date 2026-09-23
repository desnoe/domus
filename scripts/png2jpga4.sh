#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "$0")" >/dev/null 2>&1 && pwd -P)"
exec "$SCRIPT_DIR/image-convert.sh" --a4 "$@"
