// Design: docs/architecture/testing/interop.md -- ze's LAN DNS resolver against Unbound, Knot DNS and dhcpcd.
// Related: check_dns_resolver.go -- the checker bodies that run kdig and dhcpcd and hand their output here.
package bgp

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// kdigAnswer is what one kdig run printed: the response code, the header
// flags, and the answer section as "TYPE RDATA" strings, sorted. The TTL is
// left out: a cached answer from ze carries a TTL that counts down, and the
// RRset is the same RRset.
type kdigAnswer struct {
	status string
	flags  []string
	rrs    []string
}

// parseKdig reads kdig's default output. It refuses output that holds no
// header line, because a kdig that never got a reply ("no reply" after its
// retries) prints none, and an empty answer is not a reply.
func parseKdig(output string) (kdigAnswer, error) {
	var answer kdigAnswer
	header := false
	inAnswer := false
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if status, ok := strings.CutPrefix(line, ";; ->>HEADER<<- "); ok {
			for field := range strings.SplitSeq(status, ";") {
				if value, found := strings.CutPrefix(strings.TrimSpace(field), "status: "); found {
					answer.status = value
					header = true
				}
			}
			continue
		}
		if flags, ok := strings.CutPrefix(line, ";; Flags: "); ok {
			before, _, _ := strings.Cut(flags, ";")
			answer.flags = strings.Fields(before)
			continue
		}
		if strings.HasPrefix(line, ";; ANSWER SECTION:") {
			inAnswer = true
			continue
		}
		if line == "" || strings.HasPrefix(line, ";;") {
			inAnswer = false
			continue
		}
		if !inAnswer {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return kdigAnswer{}, errors.New("kdig answer line has fewer than five fields: " + line)
		}
		answer.rrs = append(answer.rrs, fields[3]+" "+strings.Join(fields[4:], " "))
	}
	if !header {
		return kdigAnswer{}, errors.New("kdig printed no reply header (no reply from the server): " + strings.TrimSpace(output))
	}
	slices.Sort(answer.rrs)
	return answer, nil
}

// dnsAnswerVerdict holds a reply to the status and RRset wanted. A nil want
// set requires an empty answer section.
func dnsAnswerVerdict(got kdigAnswer, status string, want []string) error {
	var tb textbuf.Buffer
	if got.status != status {
		return errors.New(tb.Str("status ").Str(got.status).Str(", want ").Str(status).String())
	}
	sorted := slices.Clone(want)
	slices.Sort(sorted)
	if !slices.Equal(got.rrs, sorted) {
		return errors.New(tb.Str("answer [").Str(strings.Join(got.rrs, " | ")).
			Str("], want [").Str(strings.Join(sorted, " | ")).Str("]").String())
	}
	return nil
}

// sameAnswerVerdict holds ze's reply equal to Unbound's reply to the same
// query: the same response code and the same answer RRset.
func sameAnswerVerdict(ze, unbound kdigAnswer) error {
	var tb textbuf.Buffer
	if ze.status != unbound.status {
		return errors.New(tb.Str("ze answered ").Str(ze.status).Str(", Unbound ").Str(unbound.status).String())
	}
	if !slices.Equal(ze.rrs, unbound.rrs) {
		return errors.New(tb.Str("ze answered [").Str(strings.Join(ze.rrs, " | ")).
			Str("], Unbound [").Str(strings.Join(unbound.rrs, " | ")).Str("]").String())
	}
	return nil
}

// truncatedVerdict holds a reply to carrying TC and no answer.
func truncatedVerdict(got kdigAnswer) error {
	if !slices.Contains(got.flags, "tc") {
		return errors.New("UDP reply flags [" + strings.Join(got.flags, " ") + "] carry no tc")
	}
	return nil
}

// unboundQueryCount counts the queries for name and type that Unbound logged
// from source. log-queries prints one "info: <source> <name> <type> IN" line
// per query it received (test/interop/scenarios/dns-resolver-forward-unbound/
// unbound.conf).
func unboundQueryCount(log, source, name, qtype string) int {
	var tb textbuf.Buffer
	needle := tb.Str("info: ").Str(source).Byte(' ').Str(name).Byte(' ').Str(qtype).Str(" IN").String()
	count := 0
	for line := range strings.SplitSeq(log, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), needle) {
			count++
		}
	}
	return count
}

// upstreamTruncatedOf finds the "upstream-queries" object anywhere in ze's
// decoded `show dns resolver` answer and returns its "truncated" count. An
// answer with no such object, or a count that is not a number, is refused:
// zero is a real count, and a missing field is not one.
func upstreamTruncatedOf(decoded any, raw string) (uint64, error) {
	pending := []any{decoded}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		object, ok := current.(map[string]any)
		if !ok {
			if list, isList := current.([]any); isList {
				pending = append(pending, list...)
			}
			continue
		}
		if upstream, found := object["upstream-queries"].(map[string]any); found {
			count, isNumber := upstream["truncated"].(float64)
			if !isNumber {
				return 0, errors.New("upstream-queries holds no numeric truncated count: " + raw)
			}
			return uint64(count), nil
		}
		for _, value := range object {
			pending = append(pending, value)
		}
	}
	return 0, errors.New("show dns resolver holds no upstream-queries object: " + raw)
}

// dhcpcdLease is what `dhcpcd --dumplease` printed for the bound lease.
type dhcpcdLease struct {
	address string
	servers []string
}

// parseDhcpcdLease reads dhcpcd's lease dump, one name=value line per
// option, values possibly single-quoted. It refuses a dump that holds no
// address, because dhcpcd prints nothing for an interface with no lease.
func parseDhcpcdLease(output string) (dhcpcdLease, error) {
	var lease dhcpcdLease
	for line := range strings.SplitSeq(output, "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, "'\"")
		switch name {
		case "ip_address":
			lease.address = value
		case "domain_name_servers":
			lease.servers = strings.Fields(value)
		}
	}
	if lease.address == "" {
		return dhcpcdLease{}, errors.New("dhcpcd lease dump holds no ip_address: " + strings.TrimSpace(output))
	}
	return lease, nil
}

// dhcpLeaseVerdict holds a lease to the pool address and to exactly the one
// DNS server ze's dns-server leaf names.
func dhcpLeaseVerdict(lease dhcpcdLease, address, server string) error {
	var tb textbuf.Buffer
	if lease.address != address {
		return errors.New(tb.Str("leased ").Str(lease.address).Str(", want ").Str(address).String())
	}
	if !slices.Equal(lease.servers, []string{server}) {
		return errors.New(tb.Str("option 6 carried [").Str(strings.Join(lease.servers, " ")).
			Str("], want [").Str(server).Str("]").String())
	}
	return nil
}
