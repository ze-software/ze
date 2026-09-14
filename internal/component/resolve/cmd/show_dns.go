// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- DNS lookup and cache stats (dig replacement)
// Related: dns.go -- clear dns cache (same resolve component owner)

package cmd

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"
	"time"

	mdns "github.com/miekg/dns"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/resolve/dns"
)

const (
	defaultDNSLookupTimeout = 5 * time.Second
	dnsCacheActionStats     = "stats"
	dnsCacheActionList      = "list"
	dnsCacheActionRecord    = "record"
)

// dnsLookups is the one place Go names the record types `show dns lookup`
// answers. Each word selects the standard-library lookup that serves it when
// no Ze resolver is running, and its RR type number comes from the miekg/dns
// registry (mdns.StringToType) rather than from a second copy here. The
// enumeration at show/dns/lookup/type in ze-resolve-cmd.yang offers the same
// words to an operator; TestDNSLookupTypesMatchTheModel holds the two
// together, because neither side can be derived from the other
// (ai/rules/principles.md).
// enumeration: gated by TestDNSLookupTypesMatchTheModel
var dnsLookups = map[string]dnsStdlibLookup{
	"A":     lookupA,
	"AAAA":  lookupAAAA,
	"CNAME": lookupCNAME,
	"MX":    lookupMX,
	"NS":    lookupNS,
	"PTR":   lookupPTR,
	"TXT":   lookupTXT,
}

// dnsStdlibLookup answers the records of one type for name through the
// standard-library resolver.
type dnsStdlibLookup func(ctx context.Context, resolver *net.Resolver, name string) ([]string, error)

// dnsLookupTypes answers the words dnsLookups holds, sorted, for a refusal
// that names what an operator may write.
func dnsLookupTypes() []string {
	return slices.Sorted(maps.Keys(dnsLookups))
}

// dnsLookupStdlib answers the records of qtype for name through the standard
// library. A type dnsLookups does not hold is an error rather than an empty
// answer, so a caller that skipped the table cannot read "no records" for a
// type nothing looked up.
func dnsLookupStdlib(name, qtype string) ([]string, error) {
	lookup, ok := dnsLookups[qtype]
	if !ok {
		return nil, fmt.Errorf("dns lookup: unsupported type %s (use %s)", qtype, strings.Join(dnsLookupTypes(), ", "))
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultDNSLookupTimeout)
	defer cancel()
	return lookup(ctx, &net.Resolver{}, name)
}

func lookupA(ctx context.Context, resolver *net.Resolver, name string) ([]string, error) {
	return lookupIPFamily(ctx, resolver, name, true)
}

func lookupAAAA(ctx context.Context, resolver *net.Resolver, name string) ([]string, error) {
	return lookupIPFamily(ctx, resolver, name, false)
}

// lookupIPFamily keeps the addresses of one family out of a dual-stack answer.
func lookupIPFamily(ctx context.Context, resolver *net.Resolver, name string, ipv4 bool) ([]string, error) {
	ips, err := resolver.LookupIPAddr(ctx, name)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, ip := range ips {
		if (ip.IP.To4() != nil) == ipv4 {
			out = append(out, ip.IP.String())
		}
	}
	return out, nil
}

func lookupMX(ctx context.Context, resolver *net.Resolver, name string) ([]string, error) {
	mxs, err := resolver.LookupMX(ctx, name)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, mx := range mxs {
		out = append(out, mx.Host)
	}
	return out, nil
}

func lookupNS(ctx context.Context, resolver *net.Resolver, name string) ([]string, error) {
	nss, err := resolver.LookupNS(ctx, name)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, ns := range nss {
		out = append(out, ns.Host)
	}
	return out, nil
}

func lookupTXT(ctx context.Context, resolver *net.Resolver, name string) ([]string, error) {
	return resolver.LookupTXT(ctx, name)
}

func lookupCNAME(ctx context.Context, resolver *net.Resolver, name string) ([]string, error) {
	cname, err := resolver.LookupCNAME(ctx, name)
	if err != nil {
		return nil, err
	}
	if cname != "" {
		return []string{cname}, nil
	}
	return nil, nil
}

func lookupPTR(ctx context.Context, resolver *net.Resolver, name string) ([]string, error) {
	return resolver.LookupAddr(ctx, name)
}

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:dns-lookup", Handler: handleDNSLookup},
		pluginserver.RPCRegistration{WireMethod: "ze-show:dns-cache-stats", Handler: handleDNSCacheStats},
		pluginserver.RPCRegistration{WireMethod: "ze-show:dns-cache-list", Handler: handleDNSCacheList},
		pluginserver.RPCRegistration{WireMethod: "ze-show:dns-cache-record", Handler: handleDNSCacheRecord},
	)
}

func handleDNSCacheStats(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) != 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: "dns cache stats: unexpected arguments"}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(getDNSCacheStats())}, nil
}

func handleDNSCacheList(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) != 0 {
		return &plugin.Response{Status: plugin.StatusError, Error: "dns cache list: unexpected arguments"}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(getDNSCacheEntries(""))}, nil
}

func handleDNSCacheRecord(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) != 1 || args[0] == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "dns cache record: missing name"}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(getDNSCacheEntries(args[0]))}, nil
}

func handleDNSLookup(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ""
	qtype := "A"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case argType:
			if i+1 < len(args) {
				i++
				qtype = strings.ToUpper(args[i])
			}
		default:
			if name == "" {
				name = args[i]
			}
		}
	}

	if name == "" {
		return &plugin.Response{Status: plugin.StatusError, Error: "dns lookup: missing hostname"}, nil
	}
	if len(name) > 253 {
		return &plugin.Response{Status: plugin.StatusError, Error: "dns lookup: hostname exceeds 253-character limit"}, nil
	}

	if _, ok := dnsLookups[qtype]; !ok {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "dns lookup: unsupported type " + qtype + " (use " + strings.Join(dnsLookupTypes(), ", ") + ")",
		}, nil
	}
	// The RR type number is the miekg/dns registry's, which is the one place
	// the number is declared; dnsLookups holds the word alone.
	qtypeNum, ok := mdns.StringToType[qtype]
	if !ok {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "dns lookup: type " + qtype + " has no RR type number in the DNS registry",
		}, nil
	}

	start := time.Now()
	var records []string
	var ttl uint32
	var status dns.Status
	var lookupErr error

	if resolvers != nil && resolvers.DNS != nil {
		records, ttl, status, lookupErr = resolvers.DNS.ResolveWithTTL(name, qtypeNum)
	} else {
		records, lookupErr = dnsLookupStdlib(name, qtype)
	}

	queryTime := time.Since(start)

	result := map[string]any{
		keyName:         name,
		keyType:         qtype,
		keyRecords:      records,
		keyCount:        len(records),
		"ttl":           ttl,
		"query-time-ms": float64(queryTime.Microseconds()) / 1000.0,
	}

	if lookupErr != nil {
		var dnsErr *net.DNSError
		if errors.As(lookupErr, &dnsErr) && dnsErr.IsNotFound {
			result["status"] = "NXDOMAIN"
		} else {
			result["error"] = lookupErr.Error()
		}
	}

	// The Ze resolver answers a non-success RCODE with no error, so without the
	// status this line reads exactly like a name that resolved to nothing. The
	// stdlib path above reaches the same field through net.DNSError, which is
	// the only signal it carries.
	if lookupErr == nil && status != dns.StatusUnspecified && status != dns.StatusSuccess {
		result["status"] = status.String()
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(result)}, nil
}

func getDNSCacheStats() map[string]any {
	if resolvers == nil || resolvers.DNS == nil {
		return map[string]any{
			"status": msgCacheUnavailable,
		}
	}
	s := resolvers.DNS.CacheStats()
	total := s.Hits + s.Misses
	var hitRate, missRate float64
	if total > 0 {
		hitRate = float64(s.Hits) / float64(total) * 100
		missRate = float64(s.Misses) / float64(total) * 100
	}
	return map[string]any{
		keyEntries:  s.Entries,
		"capacity":  s.Capacity,
		"hits":      s.Hits,
		"misses":    s.Misses,
		"hit-rate":  hitRate,
		"miss-rate": missRate,
		"evictions": s.Evictions,
		"expired":   s.Expired,
	}
}

func getDNSCacheEntries(filterName string) map[string]any {
	if resolvers == nil || resolvers.DNS == nil {
		return map[string]any{
			"status": msgCacheUnavailable,
		}
	}
	entries := resolvers.DNS.CacheEntries()
	all := make([]map[string]any, len(entries))
	for i, e := range entries {
		all[i] = map[string]any{
			keyName:       e.Name,
			keyType:       e.TypeName,
			keyRecords:    e.Records,
			"ttl-seconds": e.TTLSeconds,
		}
	}
	if filterName == "" {
		return map[string]any{
			keyEntries: all,
			keyCount:   len(all),
		}
	}
	filtered := make([]map[string]any, 0)
	for _, e := range all {
		if name, _ := e[keyName].(string); name == filterName {
			filtered = append(filtered, e)
		}
	}
	return map[string]any{
		keyEntries: filtered,
		keyCount:   len(filtered),
		"filter":   filterName,
	}
}
