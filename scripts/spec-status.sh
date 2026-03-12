#!/bin/bash
# Generate spec status inventory from metadata in docs/plan/spec-*.md
# Usage: scripts/spec-status.sh [--json]

set -e

cd "$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"

PLAN_DIR="docs/plan"
JSON_MODE=false
[[ "${1:-}" == "--json" ]] && JSON_MODE=true

# Extract metadata from a spec file (field name from column 2, value from column 3)
extract_field() {
    local file="$1" field="$2"
    # Only look in the first 10 lines to avoid matching other tables
    head -10 "$file" | grep "^| $field " | head -1 | awk -F'|' '{gsub(/^ +| +$/,"",$3); print $3}'
}

# Get git last-modified date
git_date() {
    git log -1 --format='%as' -- "$1" 2>/dev/null || stat -f "%Sm" -t "%Y-%m-%d" "$1" 2>/dev/null || echo "unknown"
}

# Detect spec set from filename pattern: spec-<prefix>-<N>-<name>.md
detect_set() {
    local name="$1"
    if [[ "$name" =~ ^spec-([a-z]+(-[a-z]+)*)-[0-9]+-.*\.md$ ]]; then
        echo "${BASH_REMATCH[1]}"
    else
        echo "-"
    fi
}

# Status sort order
status_order() {
    case "$1" in
        in-progress) echo 1 ;;
        ready)       echo 2 ;;
        design)      echo 3 ;;
        skeleton)    echo 4 ;;
        blocked)     echo 5 ;;
        deferred)    echo 6 ;;
        *)           echo 9 ;;
    esac
}

# Collect data
declare -a ROWS=()
for spec in "$PLAN_DIR"/spec-*.md; do
    [[ -f "$spec" ]] || continue
    [[ "$(basename "$spec")" == "spec-template.md" ]] && continue

    name=$(basename "$spec" .md | sed 's/^spec-//')
    status=$(extract_field "$spec" "Status")
    depends=$(extract_field "$spec" "Depends")
    phase=$(extract_field "$spec" "Phase")
    updated=$(extract_field "$spec" "Updated")
    git_mod=$(git_date "$spec")
    set_name=$(detect_set "$(basename "$spec")")

    # Fallback for specs without metadata yet
    [[ -z "$status" ]] && status="unknown"
    [[ -z "$depends" ]] && depends="-"
    [[ -z "$phase" ]] && phase="-"
    [[ -z "$updated" ]] && updated="$git_mod"

    order=$(status_order "$status")
    ROWS+=("${order}|${updated}|${status}|${name}|${phase}|${set_name}|${depends}|${git_mod}")
done

# Sort by status order, then by updated date (descending)
SORTED=$(printf '%s\n' "${ROWS[@]}" | sort -t'|' -k1,1n -k2,2r)

if $JSON_MODE; then
    echo "["
    first=true
    while IFS='|' read -r _ updated status name phase set_name depends git_mod; do
        $first || echo ","
        first=false
        printf '  {"name":"%s","status":"%s","depends":"%s","phase":"%s","set":"%s","updated":"%s","git-modified":"%s"}' \
            "$name" "$status" "$depends" "$phase" "$set_name" "$updated" "$git_mod"
    done <<< "$SORTED"
    echo ""
    echo "]"
else
    # Count by status
    declare -A COUNTS=()
    while IFS='|' read -r _ _ status _ _ _ _ _; do
        COUNTS[$status]=$(( ${COUNTS[$status]:-0} + 1 ))
    done <<< "$SORTED"

    TOTAL=$(echo "$SORTED" | wc -l | tr -d ' ')
    SUMMARY=""
    for s in in-progress ready design skeleton blocked deferred unknown; do
        [[ ${COUNTS[$s]:-0} -gt 0 ]] && SUMMARY="${SUMMARY:+$SUMMARY, }${COUNTS[$s]} $s"
    done
    echo "Specs: $TOTAL total ($SUMMARY)"
    echo ""

    # Table header
    printf "%-12s  %-10s  %-34s  %-5s  %-10s  %s\n" "Status" "Updated" "Spec" "Phase" "Set" "Depends"
    printf "%-12s  %-10s  %-34s  %-5s  %-10s  %s\n" "────────────" "──────────" "──────────────────────────────────" "─────" "──────────" "──────────"

    while IFS='|' read -r _ updated status name phase set_name depends _; do
        printf "%-12s  %-10s  %-34s  %-5s  %-10s  %s\n" "$status" "$updated" "$name" "$phase" "$set_name" "$depends"
    done <<< "$SORTED"
fi
