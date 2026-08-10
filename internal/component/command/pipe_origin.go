// Design: pipe.go -- pipe operator framework
// Related: pipe_resolve.go -- same pattern (PTR lookup for IP values)

package command

import (
	"context"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
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

func originJSON(v any) any {
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
			for key, value := range val {
				s, ok := value.(string)
				if ok && s != "*" && isIPAddress(s) {
					o := LookupOrigin(s)
					if o.ASN > 0 {
						val[key+"-asn"] = o.ASN
						if o.Name != "" {
							val[key+"-as-name"] = o.Name
						}
						if o.Prefix != "" {
							val[key+"-prefix"] = o.Prefix
						}
					}
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

// applyOrigin adds ASN origin data for IP address string values in JSON.
// For each string value that parses as an IP, sibling fields are added:
// "<key>-asn" (uint32), "<key>-as-name" (string), "<key>-prefix" (string).
func applyOrigin(input string) string {
	return applyJSONTransform(input, originJSON)
}
