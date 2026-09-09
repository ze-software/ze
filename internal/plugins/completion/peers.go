// Design: (none -- new feature, dynamic peer selector completion)
// Overview: main.go -- completion dispatch
// Related: words.go -- static YANG tree completion (peers.go provides dynamic peer data)

package completion

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/ze-software/ze/internal/core/bgp/asn"

	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// peers outputs tab-separated "selector\tdescription" pairs for peer completion.
// Connects to the running daemon via SSH, queries `show bgp peer list`, and outputs
// peer names, IP addresses, and deduplicated as<N> ASN selectors.
// Returns 0 on success or when daemon is unreachable (graceful fallback).
func peers() int {
	return writePeers(os.Stdout)
}

// writePeers queries the daemon and writes peer selector completions to w.
// Separated from peers() for testability.
func writePeers(w io.Writer) int {
	// NoPrompt, not LoadCredentials: completion runs with stdin on the operator's
	// terminal, so the prompting variant would ask for a password and block the
	// shell mid-completion. Missing completions are recoverable; a hung shell is not.
	creds, err := sshclient.LoadCredentialsNoPrompt()
	if err != nil {
		return 0 // Graceful fallback: no usable credentials
	}

	// ExecCommandRaw, not ExecCommand: formatPeerCompletions unmarshals this
	// answer, and the exec channel renders in the operator's configured format.
	// A table would parse as nothing and completion would silently offer no
	// peers, which is the failure this call is written to avoid.
	output, err := sshclient.ExecCommandRaw(creds, "show bgp peer list")
	if err != nil {
		return 0 // Graceful fallback: daemon not responding
	}

	return formatPeerCompletions(w, output)
}

// peerListResponse is the JSON structure returned by "show bgp peer list".
type peerListResponse struct {
	Peers map[string]peerEntry `json:"peers"`
}

// peerEntry is one peer in the list response.
//
// This process reads no configuration of its own: writePeers loads SSH
// credentials and runs a remote command, and nothing here calls asn.Configure.
// The AS number is therefore shown as the DAEMON rendered it, which asn.Number
// carries. Deriving a notation here instead would be a second declaration of
// one fact, free to disagree with the show output the operator just read.
type peerEntry struct {
	Name string `json:"name"`
	// RemoteAS is an asn.Number, not a uint32, because the daemon writes it in
	// the notation bgp/as-notation selected. Under asdot it is the string
	// "1.10", and a uint32 field fails the whole decode, which this function
	// answers by offering no completion at all.
	RemoteAS asn.Number `json:"remote-as"`
	State    string     `json:"state"`
}

// formatPeerCompletions parses the peer list JSON and writes completion pairs.
// Each peer produces up to 3 entries: name, IP, and as<N>. ASN entries are
// deduplicated (multiple peers with same ASN produce one as<N> entry).
func formatPeerCompletions(w io.Writer, jsonData string) int {
	var data peerListResponse
	if json.Unmarshal([]byte(jsonData), &data) != nil {
		return 0
	}

	// Collect sorted IPs for deterministic output.
	ips := make([]string, 0, len(data.Peers))
	for ip := range data.Peers {
		ips = append(ips, ip)
	}
	slices.Sort(ips)

	seenASN := make(map[uint32]bool)

	for _, ip := range ips {
		info := data.Peers[ip]
		// The operator completes on what they READ. asn.Number carries the
		// spelling the daemon wrote, so this process needs no notation of its
		// own and cannot disagree with the daemon about one.
		asnStr := info.RemoteAS.String()

		// Name entry
		if info.Name != "" {
			if _, err := fmt.Fprintf(w, "%s\tpeer name (%s AS %s)\n", info.Name, ip, asnStr); err != nil { //nolint:errcheck // output
				return 1
			}
		}

		// IP entry
		var desc string
		var tb textbuf.Buffer
		if info.Name != "" {
			desc = tb.Str("peer ip (").Str(info.Name).Str(" AS ").Str(asnStr).Byte(')').String()
		} else {
			desc = tb.Str("peer ip (AS ").Str(asnStr).Byte(')').String()
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\n", ip, desc); err != nil { //nolint:errcheck // output
			return 1
		}

		// ASN entry (deduplicated)
		if !seenASN[info.RemoteAS.Value()] {
			seenASN[info.RemoteAS.Value()] = true
			var asnDesc string
			if info.Name != "" {
				asnDesc = tb.Reset().Str("peer asn (").Str(info.Name).Byte(' ').Str(ip).Byte(')').String()
			} else {
				asnDesc = tb.Reset().Str("peer asn (").Str(ip).Byte(')').String()
			}
			if _, err := fmt.Fprintf(w, "as%s\t%s\n", asnStr, asnDesc); err != nil { //nolint:errcheck // output
				return 1
			}
		}
	}

	return 0
}
