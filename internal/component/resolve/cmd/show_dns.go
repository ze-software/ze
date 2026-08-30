// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- DNS lookup and cache stats (dig replacement)
// Related: dns.go -- clear dns cache (same resolve component owner)

package cmd

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

const (
	defaultDNSLookupTimeout = 5 * time.Second
	dnsCacheActionStats     = "stats"
	dnsCacheActionList      = "list"
	dnsCacheActionRecord    = "record"
)

var dnsTypeMap = map[string]uint16{
	"A":     1,
	"NS":    2,
	"CNAME": 5,
	"PTR":   12,
	"MX":    15,
	"TXT":   16,
	"AAAA":  28,
}

func dnsLookupStdlib(name, qtype string) ([]string, error) {
	resolver := &net.Resolver{}
	ctx, cancel := context.WithTimeout(context.Background(), defaultDNSLookupTimeout)
	defer cancel()

	switch qtype {
	case "A":
		ips, err := resolver.LookupIPAddr(ctx, name)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, ip := range ips {
			if ip.IP.To4() != nil {
				out = append(out, ip.IP.String())
			}
		}
		return out, nil
	case "AAAA":
		ips, err := resolver.LookupIPAddr(ctx, name)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, ip := range ips {
			if ip.IP.To4() == nil {
				out = append(out, ip.IP.String())
			}
		}
		return out, nil
	case "MX":
		mxs, err := resolver.LookupMX(ctx, name)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, mx := range mxs {
			out = append(out, mx.Host)
		}
		return out, nil
	case "NS":
		nss, err := resolver.LookupNS(ctx, name)
		if err != nil {
			return nil, err
		}
		var out []string
		for _, ns := range nss {
			out = append(out, ns.Host)
		}
		return out, nil
	case "TXT":
		return resolver.LookupTXT(ctx, name)
	case "CNAME":
		cname, err := resolver.LookupCNAME(ctx, name)
		if err != nil {
			return nil, err
		}
		if cname != "" {
			return []string{cname}, nil
		}
		return nil, nil
	case "PTR":
		return resolver.LookupAddr(ctx, name)
	default:
		return nil, nil
	}
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

	qtypeNum, ok := dnsTypeMap[qtype]
	if !ok {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "dns lookup: unsupported type " + qtype + " (use A, AAAA, MX, NS, TXT, CNAME, PTR)",
		}, nil
	}

	start := time.Now()
	var records []string
	var ttl uint32
	var lookupErr error

	if resolvers != nil && resolvers.DNS != nil {
		records, ttl, lookupErr = resolvers.DNS.ResolveWithTTL(name, qtypeNum)
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
