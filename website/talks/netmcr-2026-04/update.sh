#!/bin/sh

set -eu

DECK_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
TOOLS_DIR=$(CDPATH='' cd -- "$DECK_DIR/../../presentations/tools" && pwd)

python3 "$TOOLS_DIR/bundle-html.py" "$DECK_DIR/index.html"
