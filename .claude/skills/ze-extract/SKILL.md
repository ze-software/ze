# Extract Go Symbols

Move Go symbols (functions, types, vars, consts) from one file to another.

See also: `/ze-implement` (full spec implementation)

## Instructions

### Arguments

The user provides: `<source.go> <dest.go> <symbol1> [symbol2 ...]`

If no arguments are provided, ASK the user for the source file, destination file, and symbol names.

### Pre-Flight

1. Verify source file exists (Glob)
2. If destination exists, verify it's in the same Go package
3. Confirm the symbols exist in the source (Grep for each symbol name)
4. Show the user what will be moved and ASK for confirmation

### Extract

Build and run the extraction tool (`go run` can't be used because `.go` args confuse it):

```bash
go build -o bin/go-extract scripts/go-extract.go && bin/go-extract <source.go> <dest.go> <symbol1> [symbol2 ...]
```

### Post-Extract

1. Read both files -- verify they look correct
2. Check destination has `// Design:` annotation (add if missing -- inherit from source)
3. Check both files have `// Related:` annotations pointing to each other (add if missing)
4. Run `go vet` on the package to verify compilation
5. Report what was moved and line counts for both files

### If Destination Is New

After extraction, the new file needs:
- `//go:build` tag if source had one
- `// Design:` annotation (inherit from source, adjust topic)
- `// Related:` to source and any other siblings

### Error Recovery

If extraction fails or produces broken code:
- Read both files to understand the damage
- The source file may have had lines removed -- check `git diff` to understand what changed
- Fix manually or `git checkout` the source to restore
