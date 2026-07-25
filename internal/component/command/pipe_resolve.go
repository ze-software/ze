// Design: pipe.go -- pipe operator framework
// Related: pipe_table.go -- table rendering

package command

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// PTRResolver performs reverse DNS lookups for IP addresses.
type PTRResolver interface {
	ResolvePTR(address string) ([]string, error)
}

var (
	ptrResolver   PTRResolver
	ptrResolverMu sync.Mutex
	resolveLog    = slogutil.Logger("resolve")
)

// SetPTRResolver sets the system DNS resolver used by | resolve.
// When set, reverse lookups use the system resolver (with its LRU+TTL cache).
// When nil, falls back to net.DefaultResolver with a simple local cache.
func SetPTRResolver(r PTRResolver) {
	ptrResolverMu.Lock()
	ptrResolver = r
	ptrResolverMu.Unlock()
}

const (
	dnsTimeout  = 500 * time.Millisecond
	dnsCacheMax = 4096
)

var (
	fallbackCache   = make(map[string]string)
	fallbackCacheMu sync.Mutex
)

// ReverseLookup returns the PTR hostname for an IP, or "" on failure.
func ReverseLookup(ip string) string {
	ptrResolverMu.Lock()
	r := ptrResolver
	ptrResolverMu.Unlock()

	if r != nil {
		records, err := r.ResolvePTR(ip)
		if err != nil {
			resolveLog.Debug("PTR lookup failed", slog.String("ip", ip), slog.String("error", err.Error()))
		}
		if err == nil && len(records) > 0 {
			return strings.TrimSuffix(records[0], ".")
		}
		return ""
	}

	fallbackCacheMu.Lock()
	if name, ok := fallbackCache[ip]; ok {
		fallbackCacheMu.Unlock()
		return name
	}
	fallbackCacheMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	result := ""
	if err == nil && len(names) > 0 {
		result = strings.TrimSuffix(names[0], ".")
	}

	fallbackCacheMu.Lock()
	if len(fallbackCache) >= dnsCacheMax {
		clear(fallbackCache)
	}
	fallbackCache[ip] = result
	fallbackCacheMu.Unlock()

	return result
}

func isIPAddress(s string) bool {
	_, err := netip.ParseAddr(s)
	return err == nil
}

func resolveJSON(v any) any {
	stack := []any{v}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch val := cur.(type) {
		case []any:
			for i, item := range val {
				switch item.(type) {
				case []any, map[string]any:
					stack = append(stack, item)
				default:
					val[i] = item
				}
			}
		case map[string]any:
			for key, value := range val {
				s, ok := value.(string)
				if ok && s != "*" && isIPAddress(s) {
					val[key+"-name"] = ReverseLookup(s)
				} else {
					switch value.(type) {
					case []any, map[string]any:
						stack = append(stack, value)
					}
				}
			}
		}
	}
	return v
}

// ApplyResolve adds reverse DNS names for IP address string values in JSON.
// For each string value that parses as an IP, a sibling "<key>-name" field
// is added with the PTR result. Results are cached across invocations.
// Handles both single JSON values and NDJSON (one object per line).
func ApplyResolve(input string) string {
	return applyJSONTransform(input, resolveJSON)
}

// applyJSONTransform applies a transform function to parsed JSON data.
// Handles both single JSON values and NDJSON (one object per line).
func applyJSONTransform(input string, transform func(any) any) string {
	trimmed := strings.TrimSpace(input)

	var data any
	if err := json.Unmarshal([]byte(trimmed), &data); err == nil {
		compact := !strings.Contains(trimmed, "\n")
		data = transform(data)
		var out []byte
		var marshalErr error
		if compact {
			out, marshalErr = json.Marshal(data)
		} else {
			out, marshalErr = json.MarshalIndent(data, "", "  ")
		}
		if marshalErr != nil {
			return input
		}
		return string(out)
	}

	var sb textbuf.Buffer
	for line := range strings.SplitSeq(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			sb.Str(line).Byte('\n')
			continue
		}
		obj = transform(obj)
		out, err := json.Marshal(obj)
		if err != nil {
			sb.Str(line).Byte('\n')
			continue
		}
		sb.Write(out)
		sb.Byte('\n')
	}
	return sb.String()
}
