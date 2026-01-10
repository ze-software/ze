#!/bin/bash
# fix-lint.sh - Automated lint issue fixes for ZeBGP
# Usage: ./scripts/fix-lint.sh

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "🔧 Automated lint fixes for ZeBGP"
echo ""

# Get lint output
echo "📊 Getting lint issues..."
mkdir -p /tmp/claude/gocache /tmp/claude/golangci-cache
GOCACHE=/tmp/claude/gocache GOLANGCI_LINT_CACHE=/tmp/claude/golangci-cache \
  golangci-lint run --timeout 5m 2>&1 | tee /tmp/claude/lint-full.txt || true

# Extract issue counts
echo ""
echo "📈 Issue summary:"
grep -E "^[^ ]+\.go:[0-9]+:[0-9]+:" /tmp/claude/lint-full.txt | \
  sed 's/.*: //' | sed 's/ (.*//' | sort | uniq -c | sort -rn || true

echo ""
echo "🔨 Starting automated fixes..."
echo ""

# Fix 1: rangeValCopy - iterate by pointer
echo "1️⃣ Fixing rangeValCopy (iterate by pointer)..."
grep "rangeValCopy" /tmp/claude/lint-full.txt | \
  grep -oE "^[^:]+:[0-9]+" | \
  while IFS=: read -r file line; do
    if [ -f "$file" ]; then
      # Read the line
      content=$(sed -n "${line}p" "$file")

      # Check if it's a simple range loop: for _, x := range slice
      if echo "$content" | grep -qE "for _, [a-zA-Z]+ := range"; then
        # Extract variable name and slice
        var=$(echo "$content" | grep -oE "for _, [a-zA-Z]+" | cut -d' ' -f2)

        # Convert to index iteration
        echo "  📝 $file:$line - converting to pointer iteration"
        sed -i "" "${line}s/for _, ${var} := range \([^{]*\)/for i := range \1/" "$file"

        # Find loop body and update references (simple case - single line or block)
        next_line=$((line + 1))
        body=$(sed -n "${next_line}p" "$file")
        if echo "$body" | grep -qE "^\s+[^}]"; then
          # Update variable references in next line
          sed -i "" "${next_line}s/\b${var}\b/\&\1[i]/g" "$file"
        fi
      fi
    fi
  done

echo ""
echo "2️⃣ Fixing appendCombine (combine consecutive appends)..."
# This is complex - needs AST parsing, skipping for now
echo "  ⏭️  Skipping (requires AST analysis)"

echo ""
echo "3️⃣ Fixing simple shadow issues (rename variables)..."
grep "shadow: declaration" /tmp/claude/lint-full.txt | \
  grep -oE "^[^:]+:[0-9]+:[0-9]+" | \
  head -20 | \
  while IFS=: read -r file line col; do
    if [ -f "$file" ]; then
      # Get the line content
      content=$(sed -n "${line}p" "$file")

      # Extract shadowed variable name from error message
      shadow_msg=$(grep "^${file}:${line}:${col}:" /tmp/claude/lint-full.txt | head -1)
      var_name=$(echo "$shadow_msg" | grep -oE 'declaration of "[^"]+' | cut -d'"' -f2)

      if [ -n "$var_name" ]; then
        # Common pattern: if _, err := ...; err != nil
        if echo "$content" | grep -qE "if.*${var_name}.*:=.*${var_name}"; then
          echo "  📝 $file:$line - renaming shadowed $var_name"
          # Rename to avoid shadow (add suffix)
          sed -i "" "${line}s/\b${var_name}\b/${var_name}2/2" "$file"
        fi
      fi
    fi
  done

echo ""
echo "4️⃣ Fixing prealloc (preallocate slices)..."
grep "Consider preallocating" /tmp/claude/lint-full.txt | \
  grep -oE "^[^:]+:[0-9]+" | \
  while IFS=: read -r file line; do
    if [ -f "$file" ]; then
      content=$(sed -n "${line}p" "$file")

      # Pattern: var sliceName []Type
      if echo "$content" | grep -qE "var [a-zA-Z]+ \[\]"; then
        echo "  📝 $file:$line - needs manual review for capacity"
        echo "      $content"
      fi
    fi
  done

echo ""
echo "✅ Automated fixes complete!"
echo ""
echo "🔍 Run 'make lint' to see remaining issues"
