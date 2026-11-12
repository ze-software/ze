// Design: website/AI.md -- one commit calendar, measured once and drawn by two pages
// Related: activity.go reads git's numstat output with these, in collectActivity.
package reporewrite

import (
	"fmt"
	"strconv"
	"strings"
)

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func numstatTargetPath(path string) string {
	path = strings.TrimSpace(path)
	if !strings.Contains(path, "=>") {
		return path
	}
	if close := strings.IndexByte(path, '}'); close >= 0 && strings.Contains(path[:close], "{") {
		prefix, suffix := path[:close], path[close+1:]
		if _, target, ok := strings.Cut(prefix, "=>"); ok {
			return strings.TrimSpace(target) + suffix
		}
	}
	_, target, _ := strings.Cut(path, "=>")
	return strings.TrimSpace(target)
}

// numstatLines reads the added and deleted columns of one numstat line, whose
// two columns collectActivity has checked hold only digits; a figure too large
// for an int is refused rather than read as zero.
func numstatLines(parts []string) (added, deleted int, err error) {
	added, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("read the added lines of %s: %w", parts[2], err)
	}
	deleted, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("read the deleted lines of %s: %w", parts[2], err)
	}
	return added, deleted, nil
}
