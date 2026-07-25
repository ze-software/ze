// Design: docs/architecture/api/commands.md -- show audit handler

package show

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zeaudit "github.com/ze-software/ze/internal/core/audit"
)

var auditProvider struct {
	sync.RWMutex
	fn func(zeaudit.Filter) []zeaudit.Entry
}

// RegisterAuditProvider sets the provider used by show audit.
func RegisterAuditProvider(fn func(zeaudit.Filter) []zeaudit.Entry) {
	auditProvider.Lock()
	auditProvider.fn = fn
	auditProvider.Unlock()
}

func handleShowAudit(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	filter, parseErr := parseAuditFilter(args)
	if parseErr != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: parseErr.Error()}, nil //nolint:nilerr // operational error in Response
	}
	auditProvider.RLock()
	fn := auditProvider.fn
	auditProvider.RUnlock()
	entries := []zeaudit.Entry{}
	if fn != nil {
		entries = fn(filter)
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"entries": entries, "count": len(entries)}}, nil
}

func parseAuditFilter(args []string) (zeaudit.Filter, error) {
	var filter zeaudit.Filter
	for i := 0; i < len(args); i++ {
		if i+1 >= len(args) {
			return filter, fmt.Errorf("show audit: %s requires a value", args[i])
		}
		key := args[i]
		value := args[i+1]
		i++
		switch key {
		case "action":
			filter.Action = value
		case "actor":
			filter.Actor = value
		case "surface":
			filter.Surface = value
		case "since":
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return filter, fmt.Errorf("show audit: invalid since: %w", err)
			}
			filter.Since = parsed
		case "until":
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return filter, fmt.Errorf("show audit: invalid until: %w", err)
			}
			filter.Until = parsed
		case argCount:
			limit, err := strconv.Atoi(value)
			if err != nil || limit < 1 {
				return filter, fmt.Errorf("show audit: count must be a positive integer")
			}
			filter.Limit = limit
		default:
			return filter, fmt.Errorf("show audit: unknown filter %q", key)
		}
	}
	return filter, nil
}
