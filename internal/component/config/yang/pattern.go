// Design: docs/architecture/config/yang-config-design.md -- YANG pattern matching

package yang

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var patternCache sync.Map // string -> *patternCacheEntry

type patternCacheEntry struct {
	re  *regexp.Regexp
	err error
}

// CompilePattern compiles the YANG/XSD pattern subset Ze supports into a Go
// regexp. Results are cached so repeated calls with the same pattern (common
// when validating many list entries) avoid redundant compilation.
func CompilePattern(pattern string) (*regexp.Regexp, error) {
	if v, ok := patternCache.Load(pattern); ok {
		entry, _ := v.(*patternCacheEntry)
		return entry.re, entry.err
	}
	goPattern, err := PatternToGoRegexp(pattern)
	if err != nil {
		patternCache.Store(pattern, &patternCacheEntry{err: err})
		return nil, err
	}
	re, err := regexp.Compile(goPattern)
	if err != nil {
		err = fmt.Errorf("compile translated pattern %q: %w", goPattern, err)
		patternCache.Store(pattern, &patternCacheEntry{err: err})
		return nil, err
	}
	patternCache.Store(pattern, &patternCacheEntry{re: re})
	return re, nil
}

// MatchPattern reports whether value satisfies pattern using the supported
// YANG/XSD regex subset. YANG patterns are anchored to the full value.
func MatchPattern(pattern, value string) (bool, error) {
	re, err := CompilePattern(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(value), nil
}

// PatternToGoRegexp converts the supported YANG/XSD pattern subset to a Go
// regexp. The supported subset is intentionally conservative: constructs whose
// XSD semantics differ from Go's regexp engine are rejected at schema-use time.
func PatternToGoRegexp(pattern string) (string, error) {
	if err := rejectUnsupportedPattern(pattern); err != nil {
		return "", err
	}
	return "^(?:" + pattern + ")$", nil
}

func rejectUnsupportedPattern(pattern string) error {
	inClass := false
	escaped := false
	for i, r := range pattern {
		if escaped {
			switch r {
			case 'i', 'I', 'c', 'C', 'p', 'P':
				return fmt.Errorf("unsupported XSD regex escape \\%c in %q", r, pattern)
			}
			escaped = false
			continue
		}

		switch r {
		case '\\':
			escaped = true
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '^', '$':
			if inClass {
				continue
			}
			return fmt.Errorf("unsupported XSD regex anchor %q in %q", r, pattern)
		case '&':
			if inClass && i+1 < len(pattern) && pattern[i+1] == '&' {
				return fmt.Errorf("unsupported XSD regex character-class intersection in %q", pattern)
			}
		case '-':
			if inClass && strings.HasPrefix(pattern[i:], "-[") {
				return fmt.Errorf("unsupported XSD regex character-class subtraction in %q", pattern)
			}
		case '(':
			if i+1 < len(pattern) && pattern[i+1] == '?' {
				return fmt.Errorf("unsupported regex extension group in %q", pattern)
			}
		}
	}
	if escaped {
		return fmt.Errorf("dangling escape in pattern %q", pattern)
	}
	return nil
}
