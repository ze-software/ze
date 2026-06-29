// Design: docs/architecture/core-design.md -- audit log query and filtering
// Related: audit.go -- core types and record append

package audit

import "time"

// Filter selects audit entries for queries.
type Filter struct {
	Since   time.Time
	Until   time.Time
	Action  string
	Actor   string
	Surface string
	Limit   int
}

// Query returns audit entries matching filter, oldest first.
func (l *Log) Query(filter Filter) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	entries := make([]Entry, 0, len(l.entries))
	for _, entry := range l.entries {
		if !filter.Since.IsZero() && entry.Timestamp.Before(filter.Since) {
			continue
		}
		if !filter.Until.IsZero() && entry.Timestamp.After(filter.Until) {
			continue
		}
		if filter.Action != "" && entry.Action != filter.Action {
			continue
		}
		if filter.Actor != "" && entry.Actor != filter.Actor {
			continue
		}
		if filter.Surface != "" && entry.Surface != filter.Surface {
			continue
		}
		entries = append(entries, entry)
		if filter.Limit > 0 && len(entries) >= filter.Limit {
			break
		}
	}
	return entries
}
