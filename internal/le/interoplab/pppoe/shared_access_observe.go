// Design: docs/labs/pppoe-interop.md -- actual kernel and daemon observations
//go:build ze_l2tp

package pppoe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// These structs decode external observations, not authority or cleanup receipts.
type sharedAccessRow struct {
	SID     uint16 `json:"sid"`
	MAC     string `json:"mac"`
	Access  string `json:"interface"`
	Service string `json:"service-name"`
	State   string `json:"state"`
	Device  string `json:"ppp-interface"`
	Index   uint32 `json:"interface-index"`
	Created string `json:"created-at"`
}

type sharedAccessLink struct {
	Index  int      `json:"ifindex"`
	Name   string   `json:"ifname"`
	MAC    string   `json:"address"`
	Parent int      `json:"link_index"`
	Flags  []string `json:"flags"`
	Info   struct {
		Kind string `json:"info_kind"`
		Data struct {
			ID int `json:"id"`
		} `json:"info_data"`
	} `json:"linkinfo"`
	Addresses []struct {
		Family string `json:"family"`
		Local  string `json:"local"`
		Remote string `json:"address"`
		Bits   int    `json:"prefixlen"`
	} `json:"addr_info"`
}

type sharedAccessVLANIdentity struct {
	index int
	mac   string
}
type sharedAccessPPP struct {
	index int
	local netip.Addr
}

func sharedAccessDecodeLinks(raw []byte) ([]sharedAccessLink, error) {
	var rows []sharedAccessLink
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		return nil, errors.New("kernel link observation is null")
	}
	return rows, nil
}

func sharedAccessReadVLAN(ctx context.Context, lab interoplab.CheckerLab, peer string) (sharedAccessVLANIdentity, error) {
	result, err := lab.Exec(ctx, peer, []string{"ip", "-d", "-j", commandLink, commandShow, commandDev, sharedAccessDevice}, nil)
	if err != nil {
		return sharedAccessVLANIdentity{}, err
	}
	rows, err := sharedAccessDecodeLinks([]byte(result.Stdout))
	if err != nil {
		return sharedAccessVLANIdentity{}, err
	}
	if len(rows) != 1 {
		return sharedAccessVLANIdentity{}, errors.New("actual shared VLAN is absent or ambiguous")
	}
	row := &rows[0]
	if row.Name != sharedAccessDevice {
		return sharedAccessVLANIdentity{}, errors.New("kernel answered another access device")
	}
	if row.Index <= 0 {
		return sharedAccessVLANIdentity{}, errors.New("access has no native index")
	}
	if row.Info.Kind != "vlan" {
		return sharedAccessVLANIdentity{}, errors.New("shared access is not a native VLAN")
	}
	if row.Info.Data.ID != sharedAccessVLAN {
		return sharedAccessVLANIdentity{}, errors.New("shared access has another VLAN ID")
	}
	if !slices.Contains(row.Flags, "UP") {
		return sharedAccessVLANIdentity{}, errors.New("shared access is down")
	}
	mac, err := net.ParseMAC(row.MAC)
	if err != nil {
		return sharedAccessVLANIdentity{}, err
	}
	if len(mac) != 6 {
		return sharedAccessVLANIdentity{}, errors.New("shared access has no Ethernet MAC")
	}
	return sharedAccessVLANIdentity{row.Index, mac.String()}, nil
}

func sharedAccessReadPPP(raw []byte, device, remote string) (sharedAccessPPP, error) {
	rows, err := sharedAccessDecodeLinks(raw)
	if err != nil {
		return sharedAccessPPP{}, err
	}
	if len(rows) != 1 {
		return sharedAccessPPP{}, errors.New("named PPP link is absent or ambiguous")
	}
	row := &rows[0]
	if row.Name != device {
		return sharedAccessPPP{}, errors.New("kernel answered another PPP client")
	}
	if row.Index <= 0 {
		return sharedAccessPPP{}, errors.New("PPP link lacks a native index")
	}
	if !slices.Contains(row.Flags, "POINTOPOINT") {
		return sharedAccessPPP{}, errors.New("named client is not point-to-point")
	}
	if !slices.Contains(row.Flags, "UP") {
		return sharedAccessPPP{}, errors.New("named PPP client is down")
	}
	want, err := netip.ParseAddr(remote)
	if err != nil {
		return sharedAccessPPP{}, err
	}
	var found netip.Addr
	for _, address := range row.Addresses {
		if address.Family != "inet" {
			continue
		}
		if found.IsValid() {
			return sharedAccessPPP{}, errors.New("PPP client has multiple IPv4 assignments")
		}
		local, err := netip.ParseAddr(address.Local)
		if err != nil {
			return sharedAccessPPP{}, err
		}
		if !local.Is4() {
			return sharedAccessPPP{}, errors.New("PPP local address is not IPv4")
		}
		if local.IsUnspecified() {
			return sharedAccessPPP{}, errors.New("PPP local address is unspecified")
		}
		other, err := netip.ParseAddr(address.Remote)
		if err != nil {
			return sharedAccessPPP{}, err
		}
		if other != want {
			return sharedAccessPPP{}, errors.New("PPP client negotiated another remote")
		}
		if address.Bits != 32 {
			return sharedAccessPPP{}, errors.New("PPP remote is not a host address")
		}
		if local == other {
			return sharedAccessPPP{}, errors.New("PPP addresses are equal")
		}
		found = local
	}
	if !found.IsValid() {
		return sharedAccessPPP{}, errors.New("named PPP client has no actual IPCP assignment")
	}
	return sharedAccessPPP{row.Index, found}, nil
}

func sharedAccessRows(ctx context.Context, lab interoplab.CheckerLab) ([]sharedAccessRow, error) {
	data, err := zeRESTData(ctx, lab, "show pppoe sessions")
	if err != nil {
		return nil, err
	}
	var rows []sharedAccessRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		return nil, errors.New("daemon session observation is null")
	}
	for i := range rows {
		row := &rows[i]
		if row.SID == 0 {
			return nil, errors.New("daemon reported zero PPPoE SID")
		}
		if row.Access != sharedAccessDevice {
			return nil, errors.New("daemon admitted another access")
		}
		if row.Service != pppoeService {
			return nil, errors.New("daemon admitted another service")
		}
		if row.State != "session" {
			return nil, errors.New("daemon PPPoE session has not reached session state")
		}
		if row.Device == "" {
			return nil, errors.New("daemon PPPoE session has no native PPP interface")
		}
		if row.Index == 0 {
			return nil, errors.New("daemon PPPoE session has no native index")
		}
		mac, err := net.ParseMAC(row.MAC)
		if err != nil {
			return nil, err
		}
		if len(mac) != 6 {
			return nil, errors.New("daemon PPPoE peer has no Ethernet MAC")
		}
		row.MAC = mac.String()
	}
	return rows, nil
}

func sharedAccessRowForMAC(rows []sharedAccessRow, mac string) (sharedAccessRow, error) {
	var found []sharedAccessRow
	for _, row := range rows {
		if row.MAC == mac {
			found = append(found, row)
		}
	}
	if len(found) != 1 {
		return sharedAccessRow{}, fmt.Errorf("peer %s has %d daemon rows", mac, len(found))
	}
	return found[0], nil
}

func sharedAccessRequireSurvivor(rows []sharedAccessRow, departed, survivor sharedAccessRow) error {
	if len(rows) != 1 {
		return fmt.Errorf("remaining daemon rows=%d, want one", len(rows))
	}
	if rows[0] == departed {
		return errors.New("departed peer survived instead of selected client")
	}
	if rows[0] != survivor {
		return errors.New("surviving original PPPoE row changed")
	}
	return nil
}

func sharedAccessRequireRoute(raw []byte, device string) error {
	var rows []struct {
		Device string `json:"dev"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return err
	}
	if len(rows) != 1 {
		return errors.New("kernel route observation is absent or ambiguous")
	}
	if rows[0].Device != device {
		return fmt.Errorf("traffic uses %s instead of bound PPP interface %s", rows[0].Device, device)
	}
	return nil
}
