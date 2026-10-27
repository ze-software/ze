// Design: docs/guide/vrrp.md -- tracking changes preference, not eligibility
// Related: demos/terminal/topics/vrrp -- demotion, peer loss, takeover and preemption

package siteterminaldemo

import (
	"encoding/json"
	"fmt"
	"time"
)

type networkVRRPInstance struct {
	Interface         string   `json:"interface"`
	Group             string   `json:"group"`
	VRID              uint8    `json:"vrid"`
	Device            string   `json:"device"`
	State             string   `json:"state"`
	Priority          uint8    `json:"priority"`
	EffectivePriority uint8    `json:"effective-priority"`
	TrackedDown       []string `json:"tracked-down"`
	VirtualAddresses  []string `json:"virtual-addresses"`
}

type networkVIPOwner struct {
	device string
	mac    string
}

// Read the entire namespace: a stray VIP on the parent or another device is
// still ownership, even when the expected VRRP macvlan has no address.
func networkVIPOwners(namespace string) ([]networkVIPOwner, error) {
	args := []string{"-j"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "address", "show")
	answer, err := runCommand("ip", args, commandOptions{})
	if err != nil {
		return nil, err
	}
	return parseNetworkVIPOwners(answer)
}

func parseNetworkVIPOwners(answer []byte) ([]networkVIPOwner, error) {
	var devices []struct {
		Name      string `json:"ifname"`
		Address   string `json:"address"`
		Addresses []struct {
			Local string `json:"local"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(answer, &devices); err != nil {
		return nil, fmt.Errorf("decode kernel VIP addresses: %w", err)
	}
	var owners []networkVIPOwner
	for _, iface := range devices {
		for _, address := range iface.Addresses {
			if address.Local == "203.0.113.1" {
				owners = append(owners, networkVIPOwner{device: iface.Name, mac: iface.Address})
			}
		}
	}
	return owners, nil
}

func requireNetworkVIPOwnership(ze, peer []networkVIPOwner, masterIsZe bool, expectedDevice string) error {
	if masterIsZe {
		if len(ze) != 1 || len(peer) != 0 {
			return fmt.Errorf("VRRP Master VIP ownership: Ze=%+v keepalived=%+v", ze, peer)
		}
		if ze[0].device != expectedDevice || ze[0].mac != "00:00:5e:00:01:0c" {
			return fmt.Errorf("VRRP Master VIP device/MAC: got %+v, expected device %s and MAC 00:00:5e:00:01:0c", ze[0], expectedDevice)
		}
		return nil
	}
	if len(ze) != 0 || len(peer) != 1 {
		return fmt.Errorf("VRRP Backup VIP ownership: Ze=%+v keepalived=%+v", ze, peer)
	}
	if peer[0].mac != "00:00:5e:00:01:0c" {
		return fmt.Errorf("VRRP peer owner virtual MAC is %s", peer[0].mac)
	}
	return nil
}

func observeTopicVRRPTracking(stage string) error {
	if stage == "absent" {
		if err := requireNetworkTopicAbsent("vrrp"); err != nil {
			return err
		}
		owners, err := networkVIPOwners("")
		if err != nil {
			return err
		}
		if len(owners) != 0 {
			return fmt.Errorf("shown Ze already owns VRRP VIP before live commit")
		}
		return nil
	}
	wantState := "master"
	wantPriority := uint8(200)
	switch stage {
	case "healthy":
	case "tracked-backup", "peer-restored":
		wantState, wantPriority = "backup", 50
	case "low-priority-master":
		wantPriority = 50
	default:
		return fmt.Errorf("unknown VRRP stage %q", stage)
	}
	answer, err := networkJSON[[]networkVRRPInstance]("show vrrp interface name zeptrk0")
	if err != nil {
		return err
	}
	for _, instance := range answer {
		if instance.Interface != "zeptrk0" || instance.Group != "uplink" || instance.VRID != 12 {
			continue
		}
		tracked := len(instance.TrackedDown) == 1 && instance.TrackedDown[0] == "zetrk1"
		if instance.State != wantState || instance.Priority != 200 || instance.EffectivePriority != wantPriority || tracked != (wantPriority == 50) || (wantPriority == 200 && len(instance.TrackedDown) != 0) {
			return fmt.Errorf("VRRP %s: %+v", stage, instance)
		}
		if len(instance.VirtualAddresses) != 1 || instance.VirtualAddresses[0] != "203.0.113.1" || instance.Device == "" {
			return fmt.Errorf("VRRP group has unexpected virtual identity: %+v", instance)
		}
		zeOwners, err := networkVIPOwners("")
		if err != nil {
			return err
		}
		peerOwners, err := networkVIPOwners("showcase-vrrp")
		if err != nil {
			return err
		}
		return requireNetworkVIPOwnership(zeOwners, peerOwners, wantState == "master", instance.Device)
	}
	return fmt.Errorf("VRRP uplink group missing on zeptrk0")
}

func awaitTopicVRRPTracking(stage string) error {
	return awaitNetworkObservation("VRRP "+stage, 30*time.Second, func() error { return observeTopicVRRPTracking(stage) })
}

func requireNetworkVIPReachable(masterIsZe bool) error {
	args := []string{"-I", "zeptrk0", "-c", "3", "-W", "2", "203.0.113.1"}
	command := "ping"
	if masterIsZe {
		command = "ip"
		args = []string{"netns", "exec", "showcase-vrrp", "ping", "-I", "eth0", "-c", "3", "-W", "2", "203.0.113.1"}
	}
	_, err := runCommand(command, args, commandOptions{})
	return err
}

func checkTopicVRRPTracking() error {
	pid, err := showcasePID()
	if err != nil {
		return err
	}
	if err := observeTopicVRRPTracking("absent"); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/vrrp/1-group.conf", "topics/vrrp/2-tracking.conf"); err != nil {
		return err
	}
	if err := awaitTopicVRRPTracking("healthy"); err != nil {
		return err
	}
	if err := requireNetworkVIPReachable(true); err != nil {
		return err
	}
	for _, transition := range []struct {
		fixture    string
		verb       string
		stage      string
		masterIsZe bool
	}{
		{"tracked-uplink", "down", "tracked-backup", false},
		{"keepalived", "stop", "low-priority-master", true},
		{"keepalived", "start", "peer-restored", false},
		{"tracked-uplink", "up", "healthy", true},
	} {
		if err := controlNetworkFixture(transition.fixture, transition.verb); err != nil {
			return err
		}
		if err := awaitTopicVRRPTracking(transition.stage); err != nil {
			return err
		}
		if err := requireNetworkVIPReachable(transition.masterIsZe); err != nil {
			return err
		}
		// Reachability probes span multiple advertisements. Check exclusivity
		// again so a transient election observation cannot conceal split brain.
		if err := observeTopicVRRPTracking(transition.stage); err != nil {
			return err
		}
		if err := requireShowcasePID(pid); err != nil {
			return err
		}
	}
	statistics, err := networkJSON[[]struct {
		Interface string `json:"interface"`
		Group     string `json:"group"`
		Sent      uint64 `json:"advertisements-sent"`
		Received  uint64 `json:"advertisements-received"`
		GARPs     uint64 `json:"gratuitous-arps-sent"`
	}]("show vrrp statistics")
	if err != nil {
		return err
	}
	for _, instance := range statistics {
		if instance.Interface == "zeptrk0" && instance.Group == "uplink" && instance.Sent > 0 && instance.Received > 0 && instance.GARPs > 0 {
			return requireShowcasePID(pid)
		}
	}
	return fmt.Errorf("VRRP election lacks real received/sent advertisements and gratuitous ARPs")
}
