// Design: docs/labs/l2tp-interop.md -- independent many-session evidence.
// Related: scale.go -- live collection and assertions.
package l2tp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

type scaleSession struct {
	tunnel   uint16
	local    uint16
	remote   uint16
	username string
	address  netip.Addr
}

type scalePopulation struct {
	byUser map[string]scaleSession
}

type scaleAccounting struct {
	byUser map[string]string
}

func readScaleSessions(raw []byte, users []string) (scalePopulation, error) {
	var rows []struct {
		Tunnel   uint16 `json:"tunnel-local-tid"`
		Local    uint16 `json:"local-sid"`
		Remote   uint16 `json:"remote-sid"`
		Username string `json:"username"`
		Address  string `json:"assigned-addr"`
		State    string `json:"state"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return scalePopulation{}, fmt.Errorf("decode daemon sessions: %w", err)
	}
	if len(rows) != len(users) {
		return scalePopulation{}, fmt.Errorf("daemon has %d sessions, want %d", len(rows), len(users))
	}
	expected := make(map[string]bool, len(users))
	for _, user := range users {
		if user == "" || expected[user] {
			return scalePopulation{}, fmt.Errorf("invalid expected username %q", user)
		}
		expected[user] = true
	}
	population := scalePopulation{byUser: make(map[string]scaleSession, len(rows))}
	tuples := make(map[[2]uint16]bool, len(rows))
	addresses := make(map[netip.Addr]bool, len(rows))
	for _, row := range rows {
		if !expected[row.Username] {
			return scalePopulation{}, fmt.Errorf("unexpected or missing daemon username %q", row.Username)
		}
		if _, exists := population.byUser[row.Username]; exists {
			return scalePopulation{}, fmt.Errorf("duplicate daemon username %s", row.Username)
		}
		if row.Tunnel == 0 || row.Local == 0 || row.Remote == 0 {
			return scalePopulation{}, fmt.Errorf("session %s has zero tunnel/session identity", row.Username)
		}
		if row.State != "established" {
			return scalePopulation{}, fmt.Errorf("session %s state %s, want established", row.Username, row.State)
		}
		address, err := netip.ParseAddr(row.Address)
		if err != nil || !address.Is4() || address.IsUnspecified() {
			return scalePopulation{}, fmt.Errorf("session %s has invalid IPv4 address %q", row.Username, row.Address)
		}
		if address.Compare(netip.MustParseAddr("10.100.0.2")) < 0 || address.Compare(netip.MustParseAddr("10.100.0.100")) > 0 {
			return scalePopulation{}, fmt.Errorf("session %s address %s is outside the configured subscriber pool", row.Username, address)
		}
		tuple := [2]uint16{row.Tunnel, row.Local}
		if tuples[tuple] {
			return scalePopulation{}, fmt.Errorf("duplicate daemon tuple %v", tuple)
		}
		if addresses[address] {
			return scalePopulation{}, fmt.Errorf("duplicate daemon IPv4 address %s", address)
		}
		tuples[tuple], addresses[address] = true, true
		population.byUser[row.Username] = scaleSession{row.Tunnel, row.Local, row.Remote, row.Username, address}
	}
	return population, nil
}

func readScaleRoute(raw []byte, prefix netip.Prefix) (bool, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return false, fmt.Errorf("decode FRR route: %w", err)
	}
	if object == nil {
		return false, errors.New("FRR route answer is null")
	}
	if len(object) == 0 {
		return false, nil
	}
	var route struct {
		Prefix string `json:"prefix"`
		Count  int    `json:"pathCount"`
		Paths  []struct {
			Valid bool `json:"valid"`
			Peer  struct {
				ID string `json:"peerId"`
			} `json:"peer"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &route); err != nil {
		return false, fmt.Errorf("decode FRR path: %w", err)
	}
	actual, err := netip.ParsePrefix(route.Prefix)
	if err != nil || actual != prefix {
		return false, fmt.Errorf("FRR prefix %q, want %s", route.Prefix, prefix)
	}
	if route.Count <= 0 || route.Count != len(route.Paths) {
		return false, fmt.Errorf("FRR pathCount %d differs from %d actual paths", route.Count, len(route.Paths))
	}
	for _, path := range route.Paths {
		if path.Valid && path.Peer.ID == zeAddress {
			return true, nil
		}
	}
	return false, errors.New("FRR has no valid path received from Ze")
}

func readScaleAccounting(detail string, population scalePopulation, status string) (scaleAccounting, error) {
	result := scaleAccounting{byUser: make(map[string]string, len(population.byUser))}
	ids := make(map[string]string, len(population.byUser))
	for block := range strings.SplitSeq(strings.ReplaceAll(detail, "\r\n", "\n"), "\n\n") {
		attributes := make(map[string]string)
		for line := range strings.SplitSeq(block, "\n") {
			name, value, found := strings.Cut(strings.TrimSpace(line), " = ")
			if !found {
				continue
			}
			if name != "Acct-Status-Type" && name != "Acct-Session-Id" && name != "User-Name" && name != "Framed-IP-Address" {
				continue
			}
			if _, exists := attributes[name]; exists {
				return scaleAccounting{}, fmt.Errorf("duplicate accounting attribute %s", name)
			}
			if strings.HasPrefix(value, "\"") {
				decoded, err := strconv.Unquote(value)
				if err != nil {
					return scaleAccounting{}, fmt.Errorf("decode accounting %s: %w", name, err)
				}
				value = decoded
			}
			attributes[name] = value
		}
		if attributes["Acct-Status-Type"] != status {
			continue
		}
		user, id := attributes["User-Name"], attributes["Acct-Session-Id"]
		session, exists := population.byUser[user]
		if !exists {
			return scaleAccounting{}, fmt.Errorf("accounting %s has unknown username %q", status, user)
		}
		if id == "" {
			return scaleAccounting{}, fmt.Errorf("accounting %s for %s lacks full ID", status, user)
		}
		address, err := netip.ParseAddr(attributes["Framed-IP-Address"])
		if err != nil || address != session.address {
			return scaleAccounting{}, fmt.Errorf("accounting %s address %q differs from %s for %s", status, attributes["Framed-IP-Address"], session.address, user)
		}
		if previous, exists := result.byUser[user]; exists && previous != id {
			return scaleAccounting{}, fmt.Errorf("conflicting accounting IDs for %s", user)
		}
		if previous, exists := ids[id]; exists && previous != user {
			return scaleAccounting{}, fmt.Errorf("accounting ID %s shared by %s and %s", id, previous, user)
		}
		result.byUser[user], ids[id] = id, user
	}
	if len(result.byUser) != len(population.byUser) {
		return scaleAccounting{}, fmt.Errorf("accounting %s matches %d of %d sessions", status, len(result.byUser), len(population.byUser))
	}
	return result, nil
}

func requireScaleRemoval(before, after scalePopulation, target string) error {
	if _, exists := before.byUser[target]; !exists {
		return fmt.Errorf("removal target %s was absent", target)
	}
	if _, exists := after.byUser[target]; exists {
		return fmt.Errorf("removal target %s is still present", target)
	}
	if len(after.byUser) != len(before.byUser)-1 {
		return fmt.Errorf("removal left %d sessions, want %d", len(after.byUser), len(before.byUser)-1)
	}
	for user, session := range before.byUser {
		if user == target {
			continue
		}
		if after.byUser[user] != session {
			return fmt.Errorf("removal changed survivor %s: before %+v after %+v", user, session, after.byUser[user])
		}
	}
	return nil
}
