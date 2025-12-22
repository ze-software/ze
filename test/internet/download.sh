#!/bin/bash
# Download MRT RIB dumps and BGP4MP updates, recompress with gzip -9 for Go stdlib compatibility
#
# Usage: ./download.sh [YYYYMMDD] [HHMM]
#   YYYYMMDD - Date for RouteViews/updates (default: today)
#   HHMM     - Time for updates (default: 0000)
#
# Examples:
#   ./download.sh                # latest RIPE RIB + today's updates
#   ./download.sh 20251201       # specific date, time 0000
#   ./download.sh 20251201 1200  # specific date and time
set -e

cd "$(dirname "$0")"

DATE="${1:-$(date +%Y%m%d)}"
TIME="${2:-0000}"
MONTH="$(echo $DATE | sed 's/\(....\)\(..\).*/\1.\2/')"

# === RIPE RIS (rrc00 - Amsterdam multi-hop collector) ===

# RIB dump - always use latest
RIPE_BVIEW_URL="https://data.ris.ripe.net/rrc00/latest-bview.gz"

# BGP4MP updates - 5-minute intervals
RIPE_UPDATES_URL="https://data.ris.ripe.net/rrc00/${MONTH}/updates.${DATE}.${TIME}.gz"

# === RouteViews (route-views2) ===

# RIB dump
ROUTEVIEWS_RIB_URL="http://archive.routeviews.org/bgpdata/${MONTH}/RIBS/rib.${DATE}.${TIME}.bz2"

# BGP4MP updates - 15-minute intervals
ROUTEVIEWS_UPDATES_URL="http://archive.routeviews.org/bgpdata/${MONTH}/UPDATES/updates.${DATE}.${TIME}.bz2"

echo "=== RIPE RIS ==="

echo "Downloading latest-bview..."
curl -f -o latest-bview.gz.tmp "$RIPE_BVIEW_URL"
echo "Recompressing with gzip -9..."
gunzip -c latest-bview.gz.tmp | gzip -9 > latest-bview.gz
rm latest-bview.gz.tmp

echo "Downloading updates.${DATE}.${TIME}..."
if curl -f -o "ripe-updates.${DATE}.${TIME}.gz.tmp" "$RIPE_UPDATES_URL" 2>/dev/null; then
    echo "Recompressing with gzip -9..."
    gunzip -c "ripe-updates.${DATE}.${TIME}.gz.tmp" | gzip -9 > "ripe-updates.${DATE}.${TIME}.gz"
    rm "ripe-updates.${DATE}.${TIME}.gz.tmp"
else
    echo "⚠️  RIPE updates not available for ${DATE}.${TIME} (try different time, 5-min intervals)"
fi

echo ""
echo "=== RouteViews ==="

echo "Downloading rib.${DATE}.${TIME}..."
if curl -f -o "rib.${DATE}.${TIME}.bz2.tmp" "$ROUTEVIEWS_RIB_URL" 2>/dev/null; then
    echo "Converting bz2 to gzip -9..."
    bunzip2 -c "rib.${DATE}.${TIME}.bz2.tmp" | gzip -9 > "rib.${DATE}.${TIME}.gz"
    rm "rib.${DATE}.${TIME}.bz2.tmp"
else
    echo "⚠️  RouteViews RIB not available for ${DATE}.${TIME} (try 0000, 0200, etc.)"
fi

echo "Downloading updates.${DATE}.${TIME}..."
if curl -f -o "rv-updates.${DATE}.${TIME}.bz2.tmp" "$ROUTEVIEWS_UPDATES_URL" 2>/dev/null; then
    echo "Converting bz2 to gzip -9..."
    bunzip2 -c "rv-updates.${DATE}.${TIME}.bz2.tmp" | gzip -9 > "rv-updates.${DATE}.${TIME}.gz"
    rm "rv-updates.${DATE}.${TIME}.bz2.tmp"
else
    echo "⚠️  RouteViews updates not available for ${DATE}.${TIME} (try different time, 15-min intervals)"
fi

echo ""
echo "=== Done ==="
echo "Files:"
ls -lh *.gz 2>/dev/null || echo "No .gz files found"
