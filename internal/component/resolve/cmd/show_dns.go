// Design: plan/spec-diag-core.md -- DNS lookup and cache stats (dig replacement)
// Related: dns.go -- clear dns cache (same resolve component owner)

package cmd

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
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
		pluginserver.RPCRegistration{WireMethod: "ze-show:dns-cache", Handler: handleDNSCache},
	)
}

func handleDNSLookup(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	name := ""
	qtype := "A"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "type":
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

	if dnsLookupProvider != nil {
		res, err := dnsLookupProvider(name, qtypeNum)
		if err != nil {
			lookupErr = err
		} else if res != nil {
			records = res.Records
			ttl = res.TTL
		}
	} else {
		records, lookupErr = dnsLookupStdlib(name, qtype)
	}

	queryTime := time.Since(start)

	result := map[string]any{
		"name":          name,
		"type":          qtype,
		"records":       records,
		"count":         len(records),
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

func handleDNSCache(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	action := dnsCacheActionStats
	filterName := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case dnsCacheActionStats:
			action = dnsCacheActionStats
		case dnsCacheActionList:
			action = dnsCacheActionList
		case dnsCacheActionRecord:
			action = dnsCacheActionRecord
			if i+1 < len(args) {
				i++
				filterName = args[i]
			}
		}
	}

	switch action {
	case dnsCacheActionStats:
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(getDNSCacheStats())}, nil
	case dnsCacheActionList:
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(getDNSCacheEntries(""))}, nil
	case dnsCacheActionRecord:
		if filterName == "" {
			return &plugin.Response{Status: plugin.StatusError, Error: "dns cache record: missing name"}, nil
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(getDNSCacheEntries(filterName))}, nil
	default:
		return &plugin.Response{Status: plugin.StatusError, Error: "dns cache: unknown action (use: stats, list, record <name>)"}, nil
	}
}

var dnsStatsProvider func() map[string]any

func RegisterDNSStatsProvider(fn func() map[string]any) {
	dnsStatsProvider = fn
}

// DNSLookupResult is returned by the lookup provider when available.
type DNSLookupResult struct {
	Records []string
	TTL     uint32
}

var dnsLookupProvider func(name string, qtype uint16) (*DNSLookupResult, error)

func RegisterDNSLookupProvider(fn func(name string, qtype uint16) (*DNSLookupResult, error)) {
	dnsLookupProvider = fn
}

func getDNSCacheStats() map[string]any {
	if dnsStatsProvider != nil {
		return dnsStatsProvider()
	}
	return map[string]any{
		"status": "DNS cache not available",
	}
}

var dnsEntriesProvider func() []map[string]any

func RegisterDNSEntriesProvider(fn func() []map[string]any) {
	dnsEntriesProvider = fn
}

func getDNSCacheEntries(filterName string) map[string]any {
	if dnsEntriesProvider != nil {
		all := dnsEntriesProvider()
		if filterName == "" {
			return map[string]any{
				"entries": all,
				"count":   len(all),
			}
		}
		var filtered []map[string]any
		for _, e := range all {
			if name, _ := e["name"].(string); name == filterName {
				filtered = append(filtered, e)
			}
		}
		return map[string]any{
			"entries": filtered,
			"count":   len(filtered),
			"filter":  filterName,
		}
	}
	return map[string]any{
		"status": "DNS cache not available",
	}
}
