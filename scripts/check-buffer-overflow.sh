#!/bin/bash
# check-buffer-overflow.sh - Detect potential buffer overflow patterns in Go code
#
# Searches for risky patterns in WriteTo-style functions that write to
# pre-allocated buffers without bounds checking.
#
# Usage: ./scripts/check-buffer-overflow.sh [path]

set -euo pipefail

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

SEARCH_PATH="${1:-pkg/}"

echo -e "${CYAN}=== Buffer Overflow Pattern Scanner ===${NC}"
echo -e "Searching in: ${SEARCH_PATH}\n"

# Track findings
HIGH_RISK=0
MEDIUM_RISK=0
LOW_RISK=0

#############################################################################
# Pattern 1: WriteTo functions without bounds checking
#############################################################################
echo -e "${YELLOW}[1/6] WriteTo functions writing to buf[off+...]${NC}"
echo "Looking for direct buffer writes that could overflow..."
echo

# Find WriteTo functions and check for direct indexing
while IFS= read -r file; do
    # Extract WriteTo function bodies and look for risky patterns
    grep -n 'buf\[off' "$file" 2>/dev/null | while read -r line; do
        # Check if there's a bounds check nearby (within 10 lines before)
        linenum=$(echo "$line" | cut -d: -f1)
        start=$((linenum - 10))
        [ $start -lt 1 ] && start=1

        # Look for bounds check pattern
        if ! sed -n "${start},${linenum}p" "$file" | grep -q 'len(buf).*<\|cap(buf).*<\|checkBounds'; then
            echo -e "${RED}⚠ RISK:${NC} $file:$line"
            ((HIGH_RISK++)) || true
        fi
    done
done < <(find "$SEARCH_PATH" -name '*.go' -not -name '*_test.go')

echo

#############################################################################
# Pattern 2: binary.BigEndian writes without bounds check
#############################################################################
echo -e "${YELLOW}[2/6] binary.BigEndian.Put* without bounds check${NC}"
echo

while IFS= read -r file; do
    grep -n 'binary\.BigEndian\.Put' "$file" 2>/dev/null | while read -r line; do
        linenum=$(echo "$line" | cut -d: -f1)
        start=$((linenum - 10))
        [ $start -lt 1 ] && start=1

        if ! sed -n "${start},${linenum}p" "$file" | grep -q 'len(buf).*<\|cap(buf).*<\|checkBounds'; then
            echo -e "${YELLOW}⚠ CHECK:${NC} $file:$line"
            ((MEDIUM_RISK++)) || true
        fi
    done
done < <(find "$SEARCH_PATH" -name '*.go' -not -name '*_test.go')

echo

#############################################################################
# Pattern 3: copy() to buffer slice without length validation
#############################################################################
echo -e "${YELLOW}[3/6] copy() operations to offset slices${NC}"
echo

grep -rn 'copy(buf\[off' "$SEARCH_PATH" --include='*.go' --exclude='*_test.go' 2>/dev/null | while read -r match; do
    echo -e "${CYAN}ℹ INFO:${NC} $match"
    ((LOW_RISK++)) || true
done

echo

#############################################################################
# Pattern 4: Len() vs WriteTo mismatch risk
#############################################################################
echo -e "${YELLOW}[4/6] Functions with both Len() and WriteTo() - verify match${NC}"
echo

# Find types that have both Len() and WriteTo()
for file in $(find "$SEARCH_PATH" -name '*.go' -not -name '*_test.go'); do
    # Check if file has both Len() and WriteTo()
    if grep -q 'func.*Len().*int' "$file" && grep -q 'func.*WriteTo.*buf.*off.*int' "$file"; then
        echo -e "${CYAN}📄 Has Len()+WriteTo():${NC} $file"

        # Extract type names with WriteTo
        grep -o 'func ([a-zA-Z*]* [*]*[A-Za-z0-9_]*) WriteTo' "$file" | while read -r sig; do
            typename=$(echo "$sig" | sed 's/func (\([a-zA-Z*]* [*]*\)\([A-Za-z0-9_]*\)).*/\2/')
            echo "   → Type: $typename"
        done
    fi
done

echo

#############################################################################
# Pattern 5: Context-dependent length (ASN4 variations)
#############################################################################
echo -e "${YELLOW}[5/6] Context-dependent length functions (ASN4 risk)${NC}"
echo

grep -rn 'LenWith\|WriteToWith' "$SEARCH_PATH" --include='*.go' --exclude='*_test.go' 2>/dev/null | head -20 | while read -r match; do
    echo -e "${YELLOW}⚠ CONTEXT-DEP:${NC} $match"
done

echo

#############################################################################
# Pattern 6: Magic numbers in buffer access
#############################################################################
echo -e "${YELLOW}[6/6] Hardcoded offsets (magic numbers)${NC}"
echo

grep -rn 'buf\[off+[0-9]' "$SEARCH_PATH" --include='*.go' --exclude='*_test.go' 2>/dev/null | head -30 | while read -r match; do
    echo -e "${CYAN}ℹ MAGIC:${NC} $match"
done

echo
echo -e "${CYAN}=== Summary ===${NC}"
echo -e "High Risk:   ${RED}${HIGH_RISK}${NC}"
echo -e "Medium Risk: ${YELLOW}${MEDIUM_RISK}${NC}"
echo -e "Low Risk:    ${GREEN}${LOW_RISK}${NC}"
echo
echo -e "${CYAN}Recommendations:${NC}"
echo "1. Add 'TestLenMatchesWriteTo' to verify Len() == WriteTo() for all types"
echo "2. Consider adding checkBounds() helper for debug builds"
echo "3. Review all RISK and CHECK items manually"
