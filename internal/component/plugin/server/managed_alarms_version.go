// Design: docs/architecture/fleet-alarms.md -- deterministic software version ordering for software-behind
// Related: managed_alarms.go -- ManagedAlarmStore evaluates software-behind across declared devices

package server

import (
	"cmp"
	"slices"
	"strings"
)

const (
	softwareVersionLenMax        = 128
	softwareVersionCoreSegsMax   = 8
	softwareVersionPrePartsMax   = 16
	softwareVersionPreTokensMax  = 16
	softwareVersionCoreSegsMin   = 2
	softwareVersionMaxUint64Step = (^uint64(0)) / 10
)

type softwarePreToken struct {
	numeric bool
	num     uint64
	text    string
}

type softwarePreIdent struct {
	pureNumeric bool
	num         uint64
	tokens      []softwarePreToken
}

// parsedSoftwareVersion is a validated, comparable software release version.
// Build metadata (after '+') is validated for syntax and ignored during
// ordering, matching SemVer precedence.
type parsedSoftwareVersion struct {
	raw        string
	core       []uint64
	prerelease []softwarePreIdent
}

type uniqueSoftwareVersion struct {
	parsed  parsedSoftwareVersion
	display string
}

// parseSoftwareVersion parses comparable release strings such as "v1.2.3",
// "1.2.3", "2026.11.0", "1.2.3-rc1", and "1.2.3-rc.2+build.1". Unparseable or
// non-orderable strings such as "", "dev", or "unknown" return (zero, false).
func parseSoftwareVersion(raw string) (parsedSoftwareVersion, bool) {
	if raw == "" {
		return parsedSoftwareVersion{}, false
	}
	if len(raw) > softwareVersionLenMax {
		return parsedSoftwareVersion{}, false
	}

	s := raw
	if s[0] == 'v' || s[0] == 'V' {
		if len(s) < 2 {
			return parsedSoftwareVersion{}, false
		}
		if !isASCIIDigit(s[1]) {
			return parsedSoftwareVersion{}, false
		}
		s = s[1:]
	}

	// Separate optional build metadata (+...).
	if coreAndPre, build, hasBuild := strings.Cut(s, "+"); hasBuild {
		if !validVersionMetadata(build) {
			return parsedSoftwareVersion{}, false
		}
		s = coreAndPre
	}

	// Separate optional prerelease (-...).
	corePart, prePart, hasPre := strings.Cut(s, "-")
	coreSegs, ok := parseVersionCore(corePart)
	if !ok {
		return parsedSoftwareVersion{}, false
	}

	var preIdents []softwarePreIdent
	if hasPre {
		preIdents, ok = parseVersionPrerelease(prePart)
		if !ok {
			return parsedSoftwareVersion{}, false
		}
	}

	return parsedSoftwareVersion{
		raw:        raw,
		core:       coreSegs,
		prerelease: preIdents,
	}, true
}

func parseVersionCore(corePart string) ([]uint64, bool) {
	if corePart == "" {
		return nil, false
	}
	parts := strings.Split(corePart, ".")
	if len(parts) < softwareVersionCoreSegsMin {
		return nil, false
	}
	if len(parts) > softwareVersionCoreSegsMax {
		return nil, false
	}
	segs := make([]uint64, len(parts))
	for i, seg := range parts {
		n, ok := parseUnsignedDecimal(seg)
		if !ok {
			return nil, false
		}
		segs[i] = n
	}
	return segs, true
}

func parseVersionPrerelease(prePart string) ([]softwarePreIdent, bool) {
	if prePart == "" {
		return nil, false
	}
	parts := strings.Split(prePart, ".")
	if len(parts) > softwareVersionPrePartsMax {
		return nil, false
	}
	idents := make([]softwarePreIdent, len(parts))
	for i, part := range parts {
		ident, ok := parsePrereleaseIdent(part)
		if !ok {
			return nil, false
		}
		idents[i] = ident
	}
	return idents, true
}

func parsePrereleaseIdent(part string) (softwarePreIdent, bool) {
	if part == "" {
		return softwarePreIdent{}, false
	}
	for i := range len(part) {
		if !isVersionIdentChar(part[i]) {
			return softwarePreIdent{}, false
		}
	}
	if n, ok := parseUnsignedDecimal(part); ok {
		return softwarePreIdent{
			pureNumeric: true,
			num:         n,
		}, true
	}

	// Split alphanumeric identifiers (for example "rc10") into alternating
	// text and numeric runs so "rc2" orders before "rc10".
	tokens := make([]softwarePreToken, 0, 4)
	pos := 0
	for pos < len(part) {
		if len(tokens) >= softwareVersionPreTokensMax {
			return softwarePreIdent{}, false
		}
		start := pos
		if isASCIIDigit(part[pos]) {
			for pos < len(part) && isASCIIDigit(part[pos]) {
				pos++
			}
			n, ok := parseUnsignedDecimal(part[start:pos])
			if !ok {
				return softwarePreIdent{}, false
			}
			tokens = append(tokens, softwarePreToken{numeric: true, num: n})
			continue
		}
		for pos < len(part) && !isASCIIDigit(part[pos]) {
			pos++
		}
		tokens = append(tokens, softwarePreToken{text: strings.ToLower(part[start:pos])})
	}
	return softwarePreIdent{tokens: tokens}, true
}

func validVersionMetadata(meta string) bool {
	if meta == "" {
		return false
	}
	parts := strings.Split(meta, ".")
	if len(parts) > softwareVersionPrePartsMax {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for i := range len(part) {
			if !isVersionIdentChar(part[i]) {
				return false
			}
		}
	}
	return true
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isVersionIdentChar(b byte) bool {
	if isASCIIDigit(b) {
		return true
	}
	if b >= 'a' && b <= 'z' {
		return true
	}
	if b >= 'A' && b <= 'Z' {
		return true
	}
	return b == '-'
}

func parseUnsignedDecimal(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	var acc uint64
	for i := range len(s) {
		c := s[i]
		if !isASCIIDigit(c) {
			return 0, false
		}
		digit := uint64(c - '0')
		if acc > softwareVersionMaxUint64Step {
			return 0, false
		}
		acc *= 10
		if acc > (^uint64(0))-digit {
			return 0, false
		}
		acc += digit
	}
	return acc, true
}

// compareSoftwareVersions returns -1, 0, or +1 establishing a deterministic
// total preorder over parsedSoftwareVersion values. Trailing zero core segments
// compare equal ("1.2" == "1.2.0"), and a release ranks higher than any
// prerelease sharing the same core ("1.2.3-rc1" < "1.2.3").
func compareSoftwareVersions(a, b parsedSoftwareVersion) int {
	n := max(len(a.core), len(b.core))
	for i := range n {
		var av, bv uint64
		if i < len(a.core) {
			av = a.core[i]
		}
		if i < len(b.core) {
			bv = b.core[i]
		}
		if c := cmp.Compare(av, bv); c != 0 {
			return c
		}
	}

	aHasPre := len(a.prerelease) > 0
	bHasPre := len(b.prerelease) > 0
	if !aHasPre && !bHasPre {
		return 0
	}
	if aHasPre && !bHasPre {
		return -1
	}
	if !aHasPre && bHasPre {
		return 1
	}

	minPre := min(len(a.prerelease), len(b.prerelease))
	for i := range minPre {
		if c := comparePreIdent(a.prerelease[i], b.prerelease[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a.prerelease), len(b.prerelease))
}

func comparePreIdent(a, b softwarePreIdent) int {
	if a.pureNumeric && b.pureNumeric {
		return cmp.Compare(a.num, b.num)
	}
	// SemVer: numeric identifiers have lower precedence than non-numeric ones.
	if a.pureNumeric && !b.pureNumeric {
		return -1
	}
	if !a.pureNumeric && b.pureNumeric {
		return 1
	}
	minTok := min(len(a.tokens), len(b.tokens))
	for i := range minTok {
		at := a.tokens[i]
		bt := b.tokens[i]
		if at.numeric && bt.numeric {
			if c := cmp.Compare(at.num, bt.num); c != 0 {
				return c
			}
			continue
		}
		if at.numeric && !bt.numeric {
			return -1
		}
		if !at.numeric && bt.numeric {
			return 1
		}
		if c := cmp.Compare(at.text, bt.text); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a.tokens), len(b.tokens))
}

// buildUniqueOrderedVersions sorts and deduplicates parsed versions across the
// declared estate. Equivalent versions collapse into one entry with a
// deterministic display string so "versions behind" counts distinct releases.
func buildUniqueOrderedVersions(parsed []parsedSoftwareVersion) []uniqueSoftwareVersion {
	if len(parsed) == 0 {
		return nil
	}
	sorted := slices.Clone(parsed)
	slices.SortFunc(sorted, func(a, b parsedSoftwareVersion) int {
		if c := compareSoftwareVersions(a, b); c != 0 {
			return c
		}
		return cmp.Compare(a.raw, b.raw)
	})

	unique := make([]uniqueSoftwareVersion, 0, len(sorted))
	for _, v := range sorted {
		if len(unique) > 0 && compareSoftwareVersions(unique[len(unique)-1].parsed, v) == 0 {
			continue
		}
		unique = append(unique, uniqueSoftwareVersion{
			parsed:  v,
			display: v.raw,
		})
	}
	return unique
}

// softwareVersionRank returns the index of v in a sorted unique slice built by
// buildUniqueOrderedVersions, or -1 if absent.
func softwareVersionRank(unique []uniqueSoftwareVersion, v parsedSoftwareVersion) int {
	idx, found := slices.BinarySearchFunc(unique, v, func(entry uniqueSoftwareVersion, target parsedSoftwareVersion) int {
		return compareSoftwareVersions(entry.parsed, target)
	})
	if !found {
		return -1
	}
	return idx
}
