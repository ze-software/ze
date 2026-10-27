// Design: docs/guide/ospf.md -- Hello, database and route evidence from one live daemon
// Related: demos/terminal/topics/ospf -- timer mismatch and matching-timer recovery

package siteterminaldemo

import (
	"fmt"
	"strings"
	"time"
)

const ospfHelloDrops = "show metrics name ze_ospf_packets_dropped_total label interface frr0 label reason hello-interval"

type networkOSPFDatabase struct {
	Areas []struct {
		// AreaID is a four-octet array in the LSDB producer's JSON, not
		// the dotted-quad string used by neighbor and interface views.
		Area *[4]byte `json:"area"`
		LSAs []struct {
			Type              string `json:"type"`
			AdvertisingRouter string `json:"advertising-router"`
		} `json:"lsas"`
	} `json:"areas"`
}

func networkHasFRRRouterLSA(databases []networkOSPFDatabase) bool {
	for _, database := range databases {
		for _, area := range database.Areas {
			if area.Area == nil || *area.Area != [4]byte{} {
				continue
			}
			for _, lsa := range area.LSAs {
				if lsa.Type == "router" && lsa.AdvertisingRouter == "172.29.0.2" {
					return true
				}
			}
		}
	}
	return false
}

func observeTopicOSPF(stage string) error {
	if stage == "absent" {
		if err := requireNetworkTopicAbsent("ospf"); err != nil {
			return err
		}
		answer, err := showcaseCLI("show ospf neighbor | json | no-more")
		if err != nil && strings.Contains(answer+err.Error(), "plugin may not be running") {
			return nil
		}
		if err != nil {
			return err
		}
		if strings.TrimSpace(answer) == "[]" || strings.TrimSpace(answer) == "null" {
			return nil
		}
		return fmt.Errorf("OSPF neighbors exist before live commit: %s", answer)
	}
	if stage != "healthy" && stage != "mismatch" {
		return fmt.Errorf("unknown OSPF stage %q", stage)
	}
	neighbors, err := networkJSON[[]struct {
		Interface string `json:"interface"`
		RouterID  string `json:"router-id"`
		State     string `json:"state"`
		Address   string `json:"address"`
	}]("show ospf neighbor")
	if err != nil {
		return err
	}
	full := false
	for _, neighbor := range neighbors {
		if neighbor.Interface == "frr0" && neighbor.RouterID == "172.29.0.2" && neighbor.Address == "172.29.0.2" && neighbor.State == "full" {
			full = true
		}
	}
	routes, err := networkJSON[[]struct {
		Prefix   string `json:"prefix"`
		NextHops []struct {
			NextHop   string `json:"next-hop"`
			Interface string `json:"interface"`
		} `json:"next-hops"`
	}]("show ospf route")
	if err != nil {
		return err
	}
	route := false
	for _, entry := range routes {
		if entry.Prefix == "10.255.1.2/32" {
			for _, hop := range entry.NextHops {
				if hop.NextHop == "172.29.0.2" && hop.Interface == "frr0" {
					route = true
				}
			}
		}
	}
	interfaces, err := networkJSON[[]struct {
		Name  string `json:"name"`
		Hello uint32 `json:"hello-interval"`
		Dead  uint32 `json:"dead-interval"`
	}]("show ospf interface detail")
	if err != nil {
		return err
	}
	wantHello := uint32(10)
	if stage == "mismatch" {
		wantHello = 20
	}
	timers := false
	for _, iface := range interfaces {
		if iface.Name == "frr0" && iface.Hello == wantHello && iface.Dead == 40 {
			timers = true
		}
	}
	if stage == "mismatch" {
		drops, err := networkMetric(ospfHelloDrops)
		if err != nil {
			return err
		}
		if timers && !full && !route && drops > 0 {
			return nil
		}
		return fmt.Errorf("OSPF mismatch: timers=%t Full=%t loopback-route=%t hello-interval drops=%g", timers, full, route, drops)
	}
	databases, err := networkJSON[[]networkOSPFDatabase]("show ospf database router")
	if err != nil {
		return err
	}
	routerLSA := networkHasFRRRouterLSA(databases)
	if full && route && timers && routerLSA {
		return nil
	}
	return fmt.Errorf("OSPF healthy: Full=%t FRR Router-LSA=%t loopback-route=%t matching-timers=%t", full, routerLSA, route, timers)
}

func awaitTopicOSPF(stage string) error {
	return awaitNetworkObservation("OSPF "+stage, 80*time.Second, func() error { return observeTopicOSPF(stage) })
}

func checkTopicOSPF() error {
	pid, err := showcasePID()
	if err != nil {
		return err
	}
	if err := observeTopicOSPF("absent"); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/ospf/1-area.conf", "topics/ospf/2-interface.conf"); err != nil {
		return err
	}
	if err := awaitTopicOSPF("healthy"); err != nil {
		return err
	}
	baseline, err := networkMetric(ospfHelloDrops)
	if err != nil {
		return err
	}
	if err := requireShowcasePID(pid); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/ospf/3-mismatch.conf"); err != nil {
		return err
	}
	if err := awaitTopicOSPF("mismatch"); err != nil {
		return err
	}
	if err := awaitNetworkObservation("new OSPF Hello mismatch diagnostic", 20*time.Second, func() error {
		current, err := networkMetric(ospfHelloDrops)
		if err != nil {
			return err
		}
		if current <= baseline {
			return fmt.Errorf("hello-interval drop counter did not increase: %g <= %g", current, baseline)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := requireShowcasePID(pid); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/ospf/4-recovery.conf"); err != nil {
		return err
	}
	if err := awaitTopicOSPF("healthy"); err != nil {
		return err
	}
	return requireShowcasePID(pid)
}
