// Design: (none -- new feature, dynamic peer selector completion)
// Overview: main.go -- completion dispatch
// Related: words.go -- static YANG tree completion (peers.go provides dynamic peer data)

package completion

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
)

// peers outputs tab-separated "selector\tdescription" pairs for peer completion.
// Connects to the running daemon via SSH, queries `peer * list`, and outputs
// peer names, IP addresses, and deduplicated as<N> ASN selectors.
// Returns 0 on success or when daemon is unreachable (graceful fallback).
func peers() int {
	return writePeers(os.Stdout)
}

// writePeers queries the daemon and writes peer selector completions to w.
// Separated from peers() for testability.
func writePeers(w io.Writer) int {
	creds, err := sshclient.LoadCredentials()
	if err != nil {
		return 0 // Graceful fallback: daemon not available
	}

	output, err := sshclient.ExecCommand(creds, "peer * list")
	if err != nil {
		return 0 // Graceful fallback: daemon not responding
	}

	return formatPeerCompletions(w, output)
}

// peerListResponse is the JSON structure returned by "peer * list".
type peerListResponse struct {
	Peers map[string]peerEntry `json:"peers"`
}

// peerEntry is one peer in the list response.
type peerEntry struct {
	Name     string `json:"name"`
	RemoteAS uint32 `json:"remote-as"`
	State    string `json:"state"`
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
	sort.Strings(ips)

	seenASN := make(map[uint32]bool)

	for _, ip := range ips {
		info := data.Peers[ip]
		asnStr := strconv.FormatUint(uint64(info.RemoteAS), 10)

		// Name entry
		if info.Name != "" {
			if _, err := fmt.Fprintf(w, "%s\tpeer name (%s AS %s)\n", info.Name, ip, asnStr); err != nil {
				return 1
			}
		}

		// IP entry
		var desc string
		if info.Name != "" {
			desc = "peer ip (" + info.Name + " AS " + asnStr + ")"
		} else {
			desc = "peer ip (AS " + asnStr + ")"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\n", ip, desc); err != nil {
			return 1
		}

		// ASN entry (deduplicated)
		if !seenASN[info.RemoteAS] {
			seenASN[info.RemoteAS] = true
			var asnDesc string
			if info.Name != "" {
				asnDesc = "peer asn (" + info.Name + " " + ip + ")"
			} else {
				asnDesc = "peer asn (" + ip + ")"
			}
			if _, err := fmt.Fprintf(w, "as%s\t%s\n", asnStr, asnDesc); err != nil {
				return 1
			}
		}
	}

	return 0
}
