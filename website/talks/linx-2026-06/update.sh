#!/usr/bin/env bash
#
# Refresh the LINX 2026 presentation with current project stats.
#
# Usage: <gh-pages>/talks/linx-2026-06/update.sh [--bundle-only]

set -euo pipefail

DECK_DIR="$(cd "$(dirname "$0")" && pwd)"
TOOLS_DIR="$(cd "$DECK_DIR/../../presentations/tools" && pwd)"
SLIDES="$DECK_DIR/slides.md"

case "${1:-}" in
    "")
        ;;
    --bundle-only)
        python3 "$TOOLS_DIR/bundle-html.py" "$DECK_DIR/index.html"
        exit 0
        ;;
    *)
        echo "usage: $0 [--bundle-only]" >&2
        exit 2
        ;;
esac

eval "$("$TOOLS_DIR/update-stats.sh")"

echo "=== Generating activity ===" >&2
python3 "$TOOLS_DIR/loc_activity.py" --compact --output "$DECK_DIR/activity.html" --days 365

echo "=== Updating slides ===" >&2

comma() { printf "%'d" "$1"; }

sed -i '' "s/\*\*[0-9,]* co-authored commits\*\*/**$(comma "$COAUTHORED") co-authored commits**/" "$SLIDES"
sed -i '' "s/\*\*[0-9]* plugins\*\*/**$PLUGINS plugins**/" "$SLIDES"
sed -i '' "s/\*\*[0-9,]* config nodes\*\*/**$(comma "$YANG_NODES") config nodes**/" "$SLIDES"
sed -i '' "s/across [0-9][0-9,]* YANG schemas/across $(comma "$YANG_FILES") YANG schemas/" "$SLIDES"
sed -i '' "s/[0-9]* rationale files/$RATIONALE rationale files/" "$SLIDES"
sed -i '' "s/\*\*[0-9,]* functional tests\*\*/**$(comma "$FUNC_TESTS") functional tests**/" "$SLIDES"
sed -i '' "s/[0-9]* interop scenarios/$INTEROP interop scenarios/" "$SLIDES"
sed -i '' "s/[0-9]* learned summaries/$LEARNED learned summaries/g" "$SLIDES"
sed -i '' "s/- Only \*\*[0-9][^*]*\*\*.* of Go code/- Only **${GO_SIZE_KB}k lines** of Go code/" "$SLIDES"
sed -i '' "s/- Only \*\*[^*]*\*\* of vendored code/- Only **${VENDOR_SIZE}** of vendored code/" "$SLIDES"

echo "=== Generating inlined HTML ===" >&2
python3 "$TOOLS_DIR/bundle-html.py" "$DECK_DIR/index.html"

echo "=== Done ===" >&2
echo "Review: git -C \"$DECK_DIR\" diff -- slides.md activity.html index-inlined.html" >&2
