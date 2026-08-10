// Design: docs/architecture/core-design.md -- audit log persistence
// Related: audit.go -- core types and record append

package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Open loads or creates a JSON-lines backed audit log.
func Open(path string, maxEntries int) (*Log, error) {
	if err := validateMaxEntries(maxEntries); err != nil {
		return nil, err
	}
	log := &Log{path: path, maxEntries: maxEntries}
	if path == "" {
		return log, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // operator-configured local audit log path
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return log, nil
		}
		return nil, fmt.Errorf("read audit log: %w", err)
	}
	for line := range bytes.Lines(data) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var entry Entry
		if unmarshalErr := json.Unmarshal(line, &entry); unmarshalErr != nil {
			return nil, fmt.Errorf("read audit log entry: %w", unmarshalErr)
		}
		entry.Timestamp = entry.Timestamp.UTC()
		log.entries = append(log.entries, entry)
	}
	if len(log.entries) > maxEntries {
		log.entries = append([]Entry(nil), log.entries[len(log.entries)-maxEntries:]...)
		if persistErr := log.persistLocked(); persistErr != nil {
			return nil, persistErr
		}
	}
	return log, nil
}

func (l *Log) persistLocked() error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, entry := range l.entries {
		if err := enc.Encode(entry); err != nil {
			return fmt.Errorf("encode audit entry: %w", err)
		}
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	if err := os.Rename(tmp, l.path); err != nil {
		return fmt.Errorf("replace audit log: %w", err)
	}
	return nil
}
