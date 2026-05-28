#!/usr/bin/env bash
#
# Refresh the LINX 2026 presentation with current project stats.
#
# Usage: cd <ze-repo-root> && presentations/linx-2026-06/update.sh

set -euo pipefail

DECK_DIR="$(cd "$(dirname "$0")" && pwd)"
TOOLS_DIR="$(cd "$DECK_DIR/../tools" && pwd)"
SLIDES="$DECK_DIR/slides.md"

eval "$("$TOOLS_DIR/update-stats.sh")"

echo "=== Generating activity ===" >&2
python3 "$TOOLS_DIR/loc_activity.py" --compact --output "$DECK_DIR/activity.html" --days 365

echo "=== Updating slides ===" >&2

comma() { printf "%'d" "$1"; }

sed -i '' "s/\*\*[0-9,]* co-authored commits\*\*/**$(comma "$COAUTHORED") co-authored commits**/" "$SLIDES"
sed -i '' "s/\*\*[0-9]* plugins\*\*/**$PLUGINS plugins**/" "$SLIDES"
sed -i '' "s/\*\*[0-9,]* config nodes\*\*/**$(comma "$YANG_NODES") config nodes**/" "$SLIDES"
sed -i '' "s/[0-9]* rationale files/$RATIONALE rationale files/" "$SLIDES"
sed -i '' "s/\*\*[0-9]* functional tests\*\*/**$FUNC_TESTS functional tests**/" "$SLIDES"
sed -i '' "s/[0-9]* interop scenarios/$INTEROP interop scenarios/" "$SLIDES"
sed -i '' "s/[0-9]* learned summaries/$LEARNED learned summaries/g" "$SLIDES"
sed -i '' "s/Only \*\*[0-9]*k lines\*\*/Only **${GO_SIZE_KB}k lines**/" "$SLIDES"

echo "=== Generating inlined HTML ===" >&2
python3 "$TOOLS_DIR/bundle-html.py" "$DECK_DIR/index.html"

echo "=== Done ===" >&2
echo "Review: git diff $SLIDES" >&2
