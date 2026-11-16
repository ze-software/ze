// Design: docs/labs/l2tp-interop.md -- session-specific kernel and IPCP proof.
// Related: scale.go -- collection from each live network namespace.
package l2tp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

type scalePPPLink struct {
	name   string
	local  netip.Addr
	remote netip.Addr
}

func readScalePPPLinks(raw []byte) ([]scalePPPLink, error) {
	var rows []struct {
		Name      string   `json:"ifname"`
		Flags     []string `json:"flags"`
		Addresses []struct {
			Family string `json:"family"`
			Local  string `json:"local"`
			Remote string `json:"address"`
			Bits   int    `json:"prefixlen"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode kernel addresses: %w", err)
	}
	if rows == nil {
		return nil, errors.New("kernel address answer is null")
	}
	var result []scalePPPLink
	for index := range rows {
		row := &rows[index]
		if !slices.Contains(row.Flags, "POINTOPOINT") {
			continue
		}
		if row.Name == "" {
			return nil, errors.New("point-to-point kernel interface lacks a name")
		}
		for _, address := range row.Addresses {
			if address.Family != "inet" {
				continue
			}
			local, err := netip.ParseAddr(address.Local)
			if err != nil || !local.Is4() {
				return nil, fmt.Errorf("interface %s invalid local IPv4 %q", row.Name, address.Local)
			}
			remote, err := netip.ParseAddr(address.Remote)
			if err != nil || !remote.Is4() || address.Bits != 32 {
				return nil, fmt.Errorf("interface %s invalid point-to-point remote %q/%d", row.Name, address.Remote, address.Bits)
			}
			result = append(result, scalePPPLink{row.Name, local, remote})
		}
	}
	return result, nil
}

func requireScaleKernel(rawAddresses, rawRoutes []byte, population scalePopulation) (map[string]string, error) {
	links, err := readScalePPPLinks(rawAddresses)
	if err != nil {
		return nil, err
	}
	if len(links) != len(population.byUser) {
		return nil, fmt.Errorf("found %d PPP IPv4 links on Ze, want %d", len(links), len(population.byUser))
	}
	var routes []struct {
		Destination string `json:"dst"`
		Device      string `json:"dev"`
	}
	if err := json.Unmarshal(rawRoutes, &routes); err != nil {
		return nil, fmt.Errorf("decode kernel routes: %w", err)
	}
	if routes == nil {
		return nil, errors.New("kernel route answer is null")
	}
	result := make(map[string]string, len(population.byUser))
	used := make(map[string]bool, len(links))
	gateway := netip.MustParseAddr(localPPPAddress)
	for user, session := range population.byUser {
		for _, link := range links {
			if link.remote != session.address || link.local != gateway {
				continue
			}
			if used[link.name] {
				return nil, fmt.Errorf("PPP interface %s shared by sessions", link.name)
			}
			result[user], used[link.name] = link.name, true
		}
		device, exists := result[user]
		if !exists {
			return nil, fmt.Errorf("no Ze PPP interface for %s at %s", user, session.address)
		}
		matched := 0
		for _, route := range routes {
			if route.Destination != session.address.String() && route.Destination != netip.PrefixFrom(session.address, 32).String() {
				continue
			}
			if route.Device != device {
				return nil, fmt.Errorf("kernel route to %s uses %s, want %s", session.address, route.Device, device)
			}
			matched++
		}
		if matched != 1 {
			return nil, fmt.Errorf("kernel has %d subscriber routes for %s, want one", matched, session.address)
		}
	}
	return result, nil
}

func requireScalePeerLink(raw []byte, session scaleSession, name string) error {
	links, err := readScalePPPLinks(raw)
	if err != nil {
		return err
	}
	gateway := netip.MustParseAddr(localPPPAddress)
	for _, link := range links {
		if link.name == name && link.local == session.address && link.remote == gateway {
			return nil
		}
	}
	return fmt.Errorf("peer lacks %s with local %s and remote %s", name, session.address, localPPPAddress)
}

func requireScaleIPCP(log string, session scaleSession) error {
	var sent, received, local, remote bool
	for line := range strings.SplitSeq(log, "\n") {
		fields := strings.Fields(line)
		text := strings.Join(fields, " ")
		sent = sent || strings.Contains(text, "sent [IPCP ConfAck ")
		received = received || strings.Contains(text, "rcvd [IPCP ConfAck ")
		local = local || strings.HasSuffix(text, "local IP address "+session.address.String())
		remote = remote || strings.HasSuffix(text, "remote IP address "+localPPPAddress)
	}
	if !sent || !received || !local || !remote {
		return fmt.Errorf("%s lacks both IPCP Configure-Acks and assigned address evidence (sent=%v received=%v local=%v remote=%v)", session.username, sent, received, local, remote)
	}
	return nil
}
