// Design: docs/guide/traffic-usage.md -- attribute real packets and bound post-stop growth
// Related: demos/terminal/topics/traffic -- the recording uses these same observations

package siteterminaldemo

import (
	"fmt"
	"time"
)

type networkTrafficSnapshot struct {
	Interface  string `json:"interface"`
	IngressIPs []struct {
		IP    string `json:"ip"`
		Bytes uint64 `json:"bytes"`
	} `json:"ingress-ips"`
	IngressPorts []struct {
		Port     uint16 `json:"port"`
		Protocol string `json:"protocol"`
		Bytes    uint64 `json:"bytes"`
	} `json:"ingress-ports"`
}

type networkTrafficBytes struct {
	source uint64
	http   uint64
	icmp   uint64
}

func readNetworkTraffic() (networkTrafficBytes, error) {
	var counts networkTrafficBytes
	snapshot, err := networkJSON[networkTrafficSnapshot]("show traffic usage name traffic0")
	if err != nil {
		return counts, err
	}
	if snapshot.Interface != "traffic0" {
		return counts, fmt.Errorf("traffic0 is not the observed eBPF interface: %q", snapshot.Interface)
	}
	for _, source := range snapshot.IngressIPs {
		if source.IP == "10.77.0.2" {
			counts.source += source.Bytes
		}
	}
	for _, port := range snapshot.IngressPorts {
		if port.Protocol == "tcp" && port.Port == 8080 {
			counts.http += port.Bytes
		}
		if port.Protocol == "icmp" && port.Port == 0 {
			counts.icmp += port.Bytes
		}
	}
	return counts, nil
}

func requireNetworkTrafficAccounted() error {
	counts, err := readNetworkTraffic()
	if err != nil {
		return err
	}
	if counts.source == 0 || counts.http == 0 || counts.icmp == 0 {
		return fmt.Errorf("traffic attribution missing source 10.77.0.2, TCP/8080 or ICMP: %+v", counts)
	}
	return nil
}

func awaitNetworkTrafficIncrease(baseline networkTrafficBytes) error {
	return awaitNetworkObservation("real traffic burst counters", 15*time.Second, func() error {
		counts, err := readNetworkTraffic()
		if err != nil {
			return err
		}
		if counts.source <= baseline.source || counts.http <= baseline.http || counts.icmp <= baseline.icmp {
			return fmt.Errorf("burst must increase each attributed counter: before %+v, now %+v", baseline, counts)
		}
		return nil
	})
}

func awaitNetworkTrafficSettled() error {
	previous, err := readNetworkTraffic()
	if err != nil {
		return err
	}
	if err := requireNetworkTrafficAccounted(); err != nil {
		return err
	}
	stableSince := time.Now()
	return awaitNetworkObservation("traffic counters settle after senders stop", 10*time.Second, func() error {
		current, err := readNetworkTraffic()
		if err != nil {
			return err
		}
		if current.source < previous.source || current.http < previous.http || current.icmp < previous.icmp {
			return fmt.Errorf("cumulative traffic counters reset after stop: previous %+v, now %+v", previous, current)
		}
		if current != previous {
			previous, stableSince = current, time.Now()
		}
		if time.Since(stableSince) < 2*time.Second {
			return fmt.Errorf("waiting for two seconds with no additional attributed packets: %+v", current)
		}
		return nil
	})
}

func awaitTopicTrafficUsage(stage string) error {
	switch stage {
	case stageAbsent:
		if err := requireNetworkTopicAbsent("traffic"); err != nil {
			return err
		}
		answer, err := networkJSON[struct {
			Status string `json:"status"`
		}]("show traffic usage")
		if err != nil {
			return err
		}
		if answer.Status != "not-configured" {
			return fmt.Errorf("traffic usage already active: status %q", answer.Status)
		}
		return nil
	case "configured":
		return awaitNetworkObservation("traffic0 eBPF attachment", 15*time.Second, func() error { _, err := readNetworkTraffic(); return err })
	case "accounted":
		return awaitNetworkObservation("traffic attribution", 15*time.Second, requireNetworkTrafficAccounted)
	case "increasing":
		baseline, err := readNetworkTraffic()
		if err != nil {
			return err
		}
		return awaitNetworkTrafficIncrease(baseline)
	case "settled":
		return awaitNetworkTrafficSettled()
	default:
		return fmt.Errorf("unknown traffic stage %q", stage)
	}
}

func checkTopicTrafficUsage() error {
	pid, err := showcasePID()
	if err != nil {
		return err
	}
	if err := awaitTopicTrafficUsage(stageAbsent); err != nil {
		return err
	}
	if err := loadShowcaseSections("topics/traffic/1-accounting.conf", "topics/traffic/2-interface.conf"); err != nil {
		return err
	}
	if err := awaitTopicTrafficUsage("configured"); err != nil {
		return err
	}
	for _, args := range [][]string{
		{ipNetns, commandExec, "traffic-peer", programPing, "-c", "3", "-W", "2", "10.77.0.1"},
		{ipNetns, commandExec, "traffic-peer", programCurl, "--fail", "--silent", "--show-error", curlMaxTime, "5", "--output", "/dev/null", "http://10.77.0.1:8080/payload.txt"},
	} {
		if _, err := runCommand("ip", args, commandOptions{}); err != nil {
			return err
		}
	}
	if err := awaitTopicTrafficUsage("accounted"); err != nil {
		return err
	}
	baseline, err := readNetworkTraffic()
	if err != nil {
		return err
	}
	if err := requireShowcasePID(pid); err != nil {
		return err
	}
	if err := controlNetworkFixture(showcaseTraffic, "burst"); err != nil {
		return err
	}
	if err := awaitNetworkTrafficIncrease(baseline); err != nil {
		return err
	}
	if err := awaitTopicTrafficUsage("increasing"); err != nil {
		return err
	}
	for _, command := range []string{
		"show metrics name ze_traffic_usage_ingress_bytes_total label interface traffic0 label src_ip 10.77.0.2",
		"show metrics name ze_traffic_usage_ingress_port_bytes_total label interface traffic0 label dst_port 8080 label protocol tcp",
		"show metrics name ze_traffic_usage_ingress_port_bytes_total label interface traffic0 label dst_port 0 label protocol icmp",
	} {
		if err := awaitNetworkObservation("traffic exported metric", 5*time.Second, func() error {
			value, err := networkMetric(command)
			if err != nil {
				return err
			}
			if value <= 0 {
				return fmt.Errorf("%s is not positive: %g", command, value)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	if err := requireShowcasePID(pid); err != nil {
		return err
	}
	if err := controlNetworkFixture(showcaseTraffic, controlStop); err != nil {
		return err
	}
	if err := awaitTopicTrafficUsage("settled"); err != nil {
		return err
	}
	return requireShowcasePID(pid)
}
