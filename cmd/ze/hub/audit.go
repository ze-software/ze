// Design: docs/architecture/core-design.md -- audit log file management

package hub

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/audit"
)

func openAuditLog(configPath string) (*audit.Log, error) {
	path := defaultAuditPath(configPath)
	if path != "" {
		dir := filepath.Dir(path)
		if dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, fmt.Errorf("create audit log dir: %w", err)
			}
		}
	}
	log, err := audit.Open(path, audit.DefaultMaxEntries)
	if err != nil {
		return nil, err
	}
	return log, nil
}

func defaultAuditPath(configPath string) string {
	if configPath == "" || configPath == "-" {
		return ""
	}
	base := filepath.Base(configPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		name = "ze"
	}
	return filepath.Join(filepath.Dir(configPath), name+".audit.jsonl")
}
