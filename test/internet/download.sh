#!/bin/bash
# Download MRT RIB dumps and recompress with gzip -9 for Go stdlib compatibility
#
# Usage: ./download.sh [YYYYMMDD]
#   YYYYMMDD - RouteViews date (default: first of current month)
#
# Examples:
#   ./download.sh           # latest RIPE + current month RouteViews
#   ./download.sh 20251201  # latest RIPE + specific RouteViews date
set -e

cd "$(dirname "$0")"

# RIPE RIS - Amsterdam collector (rrc00)
# Always available as "latest-bview.gz"
RIPE_URL="https://data.ris.ripe.net/rrc00/latest-bview.gz"

# RouteViews - route-views2
# Format: YYYY.MM/RIBS/rib.YYYYMMDD.HHMM.bz2
# Default to first of current month at 00:00
ROUTEVIEWS_DATE="${1:-$(date +%Y%m01)}"
ROUTEVIEWS_MONTH="$(echo $ROUTEVIEWS_DATE | sed 's/\(....\)\(..\).*/\1.\2/')"
ROUTEVIEWS_URL="http://archive.routeviews.org/bgpdata/${ROUTEVIEWS_MONTH}/RIBS/rib.${ROUTEVIEWS_DATE}.0000.bz2"

echo "Downloading RIPE RIS latest-bview..."
curl -o latest-bview.gz.tmp "$RIPE_URL"

echo "Recompressing latest-bview with gzip -9..."
gunzip -c latest-bview.gz.tmp | gzip -9 > latest-bview.gz
rm latest-bview.gz.tmp

echo "Downloading RouteViews rib.${ROUTEVIEWS_DATE}.0000..."
curl -o "rib.${ROUTEVIEWS_DATE}.0000.bz2.tmp" "$ROUTEVIEWS_URL"

echo "Converting bz2 to gzip -9..."
bunzip2 -c "rib.${ROUTEVIEWS_DATE}.0000.bz2.tmp" | gzip -9 > "rib.${ROUTEVIEWS_DATE}.0000.gz"
rm "rib.${ROUTEVIEWS_DATE}.0000.bz2.tmp"

echo ""
echo "Done. Files:"
ls -lh *.gz
