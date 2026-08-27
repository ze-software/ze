// Design: pipe.go -- pipe operator framework
// Related: pipe_resolve.go -- same pattern (PTR lookup for IP values)

package command

import (
	"context"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// OriginResult holds the ASN origin data for an IP address.
type OriginResult struct {
	ASN    uint32
	Prefix string
	Name   string
}

// OriginResolver looks up the origin ASN for an IP address.
type OriginResolver interface {
	LookupOrigin(ctx context.Context, ip string) (OriginResult, error)
}

var (
	originResolver   OriginResolver
	originResolverMu sync.Mutex
	originLog        = slogutil.Logger("origin")
)

// SetOriginResolver sets the resolver used by | origin.
func SetOriginResolver(r OriginResolver) {
	originResolverMu.Lock()
	originResolver = r
	originResolverMu.Unlock()
}

const originTimeout = 2 * time.Second

// LookupOrigin returns the ASN origin for an IP, or empty result on failure.
func LookupOrigin(ip string) OriginResult {
	originResolverMu.Lock()
	r := originResolver
	originResolverMu.Unlock()

	if r == nil {
		return OriginResult{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), originTimeout)
	defer cancel()

	result, err := r.LookupOrigin(ctx, ip)
	if err != nil {
		originLog.Debug("origin lookup failed", "ip", ip, "error", err.Error())
		return OriginResult{}
	}
	return result
}

func originJSON(v any, fields []string) any {
	stack := []any{v}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch val := cur.(type) {
		case []any:
			for _, item := range val {
				switch item.(type) {
				case []any, map[string]any:
					stack = append(stack, item)
				}
			}
		case map[string]any:
			var derivedKey textbuf.Buffer
			for key, value := range val {
				s, isString := value.(string)
				if isString {
					if declaredAddressField(fields, key) {
						if s != "*" {
							if isIPAddress(s) {
								o := LookupOrigin(s)
								if o.ASN > 0 {
									asnKey := derivedKey.Reset().Str(key).Str("-asn").String()
									val[asnKey] = o.ASN
									if o.Name != "" {
										nameKey := derivedKey.Reset().Str(key).Str("-as-name").String()
										val[nameKey] = o.Name
									}
									if o.Prefix != "" {
										prefixKey := derivedKey.Reset().Str(key).Str("-prefix").String()
										val[prefixKey] = o.Prefix
									}
								}
							}
						}
					}
					continue
				}
				switch value.(type) {
				case []any, map[string]any:
					stack = append(stack, value)
				}
			}
		}
	}
	return v
}

// applyOrigin adds ASN origin data for declared IP address fields in JSON.
// For each matching string value, sibling fields are added: "<key>-asn"
// (uint32), "<key>-as-name" (string), and "<key>-prefix" (string).
func applyOrigin(input string, fields []string) string {
	return applyJSONTransform(input, func(v any) any {
		return originJSON(v, fields)
	})
}
