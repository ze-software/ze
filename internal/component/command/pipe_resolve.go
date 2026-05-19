// Design: pipe.go -- pipe operator framework
// Related: pipe_table.go -- table rendering

package command

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const (
	dnsTimeout  = 500 * time.Millisecond
	dnsCacheMax = 4096
)

var (
	dnsCache   = make(map[string]string)
	dnsCacheMu sync.Mutex
)

func reverseLookup(ip string) string {
	dnsCacheMu.Lock()
	if name, ok := dnsCache[ip]; ok {
		dnsCacheMu.Unlock()
		return name
	}
	dnsCacheMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	result := ""
	if err == nil && len(names) > 0 {
		result = strings.TrimSuffix(names[0], ".")
	}

	dnsCacheMu.Lock()
	if len(dnsCache) >= dnsCacheMax {
		clear(dnsCache)
	}
	dnsCache[ip] = result
	dnsCacheMu.Unlock()

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
					val[key+"-name"] = reverseLookup(s)
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
	trimmed := strings.TrimSpace(input)

	var data any
	if err := json.Unmarshal([]byte(trimmed), &data); err == nil {
		compact := !strings.Contains(trimmed, "\n")
		data = resolveJSON(data)
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

	var sb strings.Builder
	for line := range strings.SplitSeq(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			sb.WriteString(line)
			sb.WriteByte('\n')
			continue
		}
		obj = resolveJSON(obj)
		out, err := json.Marshal(obj)
		if err != nil {
			sb.WriteString(line)
			sb.WriteByte('\n')
			continue
		}
		sb.Write(out)
		sb.WriteByte('\n')
	}
	return sb.String()
}
