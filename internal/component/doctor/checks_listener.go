// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Related: doctor.go — readiness check runner and output contract
// Related: checks_helpers.go — shared config-tree navigation helpers

// Listener readiness checks: collect every configured listen endpoint
// (schema-discovered plus protocol-specific extractors) and probe that the
// address/port can be bound; plus DHCP listen-interface existence.

package doctor

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorListenerFailEnv forces selected listener codes to fail in tests.
const doctorListenerFailEnv = "ze.test.doctor.listener-fail-code"

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorListenerFailEnv,
	Type:        envTypeString,
	Description: "Force selected doctor listener codes to fail (test infrastructure)",
	Private:     true,
})

type serviceListener struct {
	service  string
	network  string
	host     string
	port     string
	code     string
	severity diagnostic.Severity
}

var listenerProbe = probeListener

var registerListenerDefaultsOnce sync.Once

func collectSchemaListeners(tree *config.Tree) []serviceListener {
	schema, err := config.YANGSchema()
	if err != nil {
		return collectHardcodedListeners(tree)
	}
	if len(config.DiscoverListenerServices(schema)) == 0 {
		return collectHardcodedListeners(tree)
	}
	registerListenerDefaultsOnce.Do(config.RegisterBuiltinListenerDefaults)
	endpoints := config.CollectListenersWithDefaults(tree, schema)

	listeners := make([]serviceListener, 0, len(endpoints))
	for _, ep := range endpoints {
		l := serviceListener{
			service:  ep.Service,
			network:  ep.Protocol,
			host:     ep.IP.String(),
			port:     textbuf.StringUint16(ep.Port),
			code:     "doctor-listen-unavailable",
			severity: diagnostic.SeverityWarning,
		}
		listeners = append(listeners, l)
	}
	return listeners
}

func collectHardcodedListeners(tree *config.Tree) []serviceListener {
	var listeners []serviceListener

	if webCfg, ok := config.ExtractWebConfig(tree); ok {
		for _, s := range webCfg.Servers {
			listeners = append(listeners, tcpListener("web", s.Host, s.Port, "doctor-listen-unavailable"))
		}
	}
	if mcpCfg, ok := config.ExtractMCPConfig(tree); ok {
		for _, s := range mcpCfg.Servers {
			listeners = append(listeners, tcpListener("mcp", s.Host, s.Port, "doctor-listen-unavailable"))
		}
	}
	if gnmiCfg, ok := config.ExtractGNMIConfig(tree); ok {
		for _, s := range gnmiCfg.Servers {
			listeners = append(listeners, tcpListener("gnmi", s.Host, s.Port, "doctor-listen-unavailable"))
		}
	}
	if lgCfg, ok := config.ExtractLGConfig(tree); ok {
		for _, s := range lgCfg.Servers {
			listeners = append(listeners, tcpListener("looking-glass", s.Host, s.Port, "doctor-listen-unavailable"))
		}
	}
	if apiCfg, ok := config.ExtractAPIConfig(tree); ok {
		if apiCfg.RESTOn {
			for _, s := range apiCfg.REST {
				listeners = append(listeners, tcpListener("api-server-rest", s.Host, s.Port, "doctor-listen-unavailable"))
			}
		}
		if apiCfg.GRPCOn {
			for _, s := range apiCfg.GRPC {
				listeners = append(listeners, tcpListener("api-server-grpc", s.Host, s.Port, "doctor-listen-unavailable"))
			}
		}
	}
	listeners = append(listeners, extractSSHListeners(tree)...)
	listeners = append(listeners, extractTelemetryListeners(tree)...)
	return listeners
}

func extractSSHListeners(tree *config.Tree) []serviceListener {
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return nil
	}
	sshBlock := envBlock.GetContainer("ssh")
	if sshBlock == nil {
		return nil
	}
	enabled, _ := sshBlock.Get("enabled")
	if enabled != configTrueValue {
		return nil
	}

	var listeners []serviceListener
	if servers := sshBlock.GetListOrdered("server"); len(servers) > 0 {
		for _, s := range servers {
			host := "0.0.0.0"
			port := "2222"
			if v, ok := s.Value.Get("ip"); ok && v != "" {
				host = v
			}
			if v, ok := s.Value.Get("port"); ok && v != "" {
				port = v
			}
			listeners = append(listeners, tcpListener("ssh", host, port, "doctor-listen-unavailable"))
		}
	}

	if len(listeners) == 0 {
		listeners = append(listeners, tcpListener("ssh", "127.0.0.1", "2222", "doctor-listen-unavailable"))
	}

	return listeners
}

func extractTelemetryListeners(tree *config.Tree) []serviceListener {
	prom := getContainerPath(tree, "telemetry", "prometheus")
	if !configEnabled(prom, false) {
		return nil
	}

	servers := prom.GetListOrdered("server")
	if len(servers) == 0 {
		return []serviceListener{tcpListener("telemetry", "127.0.0.1", "9273", "doctor-listen-unavailable")}
	}

	listeners := make([]serviceListener, 0, len(servers))
	for _, s := range servers {
		host := valueOrDefault(s.Value, "ip", "127.0.0.1")
		port := valueOrDefault(s.Value, "port", "9273")
		listeners = append(listeners, tcpListener("telemetry", host, port, "doctor-listen-unavailable"))
	}
	return listeners
}

// collectAllListeners returns every endpoint ze doctor probes for a config: the
// schema-discovered ze:listener services, plus the protocol listeners that have
// no schema entry and come from the extractors below.
//
// Separated from checkListeners so the set can be inspected without binding
// sockets. The dependency inventory (doctor_test.go) reads it to prove each row
// it claims is produced by live code.
func collectAllListeners(tree *config.Tree) []serviceListener {
	listeners := collectSchemaListeners(tree)
	listeners = append(listeners, extractBGPListeners(tree)...)
	listeners = append(listeners, extractBFDListeners(tree)...)
	listeners = append(listeners, extractIPsecListeners(tree)...)
	listeners = append(listeners, extractTFTPListeners(tree)...)
	listeners = append(listeners, extractImageListeners(tree)...)
	listeners = append(listeners, extractNTPListeners(tree)...)
	return dedupeListeners(listeners)
}

func checkListeners(tree *config.Tree) []diagnostic.Diagnostic {
	var tb textbuf.Buffer
	var diags []diagnostic.Diagnostic

	for _, l := range collectAllListeners(tree) {
		if err := listenerProbe(l); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     l.code,
				Severity: l.severity,
				Message:  tb.Reset().Str(l.service).Str(": cannot bind ").Str(l.network).Byte(' ').Str(listenerAddress(l)).Str(": ").Err(err).String(),
			})
		}
	}

	return diags
}

func tcpListener(service, host, port, code string) serviceListener {
	return serviceListener{service: service, network: "tcp", host: host, port: port, code: code, severity: diagnostic.SeverityWarning}
}

func udpListener(service, port, code string) serviceListener {
	return serviceListener{service: service, network: "udp", host: "0.0.0.0", port: port, code: code, severity: diagnostic.SeverityWarning}
}

func probeListener(l serviceListener) error {
	if forcedDoctorCode(l.code, env.Get(doctorListenerFailEnv)) {
		return errors.New("forced listener failure")
	}

	addr := listenerAddress(l)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lc net.ListenConfig
	if l.network == "udp" {
		pc, err := lc.ListenPacket(ctx, l.network, addr)
		if err != nil {
			return err
		}
		return pc.Close()
	}

	ln, err := lc.Listen(ctx, l.network, addr)
	if err != nil {
		return err
	}
	return ln.Close()
}

func listenerAddress(l serviceListener) string {
	return net.JoinHostPort(l.host, l.port)
}

func dedupeListeners(listeners []serviceListener) []serviceListener {
	var tb textbuf.Buffer
	seen := make(map[string]bool, len(listeners))
	result := make([]serviceListener, 0, len(listeners))
	for _, l := range listeners {
		if l.code == "" {
			l.code = "doctor-listen-unavailable"
		}
		if l.severity == "" {
			l.severity = diagnostic.SeverityWarning
		}
		key := tb.Reset().Str(l.service).Byte(0).Str(l.network).Byte(0).Str(l.host).Byte(0).Str(l.port).Byte(0).Str(l.code).String()
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, l)
	}
	return result
}

func extractBGPListeners(tree *config.Tree) []serviceListener {
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil
	}

	var listeners []serviceListener
	for _, p := range bgp.GetListOrdered("peer") {
		listeners = appendBGPListener(listeners, nil, p.Value)
	}
	for _, g := range bgp.GetListOrdered("group") {
		if remoteIP, _ := nestedValue(g.Value, "connection", "remote", "ip"); remoteIP == "dynamic" {
			listeners = appendBGPListener(listeners, nil, g.Value)
		}
		for _, p := range g.Value.GetListOrdered("peer") {
			listeners = appendBGPListener(listeners, g.Value, p.Value)
		}
	}
	return listeners
}

func appendBGPListener(listeners []serviceListener, parent, node *config.Tree) []serviceListener {
	if accept, ok := inheritedValue(parent, node, "connection", "local", "accept"); ok && accept == "false" {
		return listeners
	}

	host, ok := inheritedValue(parent, node, "connection", "local", "ip")
	if !ok || host == "" || host == "auto" {
		return listeners
	}

	port, _ := inheritedValue(parent, node, "connection", "local", "port")
	if port == "" {
		port, _ = inheritedValue(parent, node, "connection", "remote", "port")
	}
	if port == "" {
		port = "179"
	}

	return append(listeners, tcpListener("bgp", host, port, "doctor-bgp-listen"))
}

func extractBFDListeners(tree *config.Tree) []serviceListener {
	bfd := tree.GetContainer("bfd")
	if !configEnabled(bfd, true) {
		return nil
	}
	return []serviceListener{udpListener("bfd", "3784", "doctor-bfd-port")}
}

func extractIPsecListeners(tree *config.Tree) []serviceListener {
	if getContainerPath(tree, "vpn", "ipsec") == nil {
		return nil
	}
	return []serviceListener{
		udpListener("ipsec", "500", "doctor-ipsec-listen"),
		udpListener("ipsec", "4500", "doctor-ipsec-listen"),
	}
}

func extractTFTPListeners(tree *config.Tree) []serviceListener {
	tftp := getContainerPath(tree, "service", "tftp-server")
	if !configEnabled(tftp, false) {
		return nil
	}
	return []serviceListener{udpListener("tftp", "69", "doctor-tftp-listen")}
}

func extractImageListeners(tree *config.Tree) []serviceListener {
	image := getContainerPath(tree, "service", "image-server")
	if !configEnabled(image, false) {
		return nil
	}
	return []serviceListener{tcpListener("image-server", "0.0.0.0", valueOrDefault(image, "listen-port", "80"), "doctor-image-listen")}
}

func extractNTPListeners(tree *config.Tree) []serviceListener {
	ntp := getContainerPath(tree, "environment", "ntp")
	if !configEnabled(ntp, false) {
		return nil
	}
	return []serviceListener{udpListener("ntp", "123", "doctor-ntp-listen")}
}

var interfaceByName = net.InterfaceByName

func checkDHCPInterfaces(tree *config.Tree) []diagnostic.Diagnostic {
	dhcp := getContainerPath(tree, "service", "dhcp-server")
	if !configEnabled(dhcp, false) {
		return nil
	}

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for _, name := range dhcp.GetSlice("listen-interface") {
		if strings.ContainsAny(name, "/\x00") || strings.Contains(name, "..") {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-dhcp-iface",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("DHCP server listen interface has invalid name: ").Str(name).String(),
				Path:     "service/dhcp-server/listen-interface",
			})
			continue
		}
		if _, err := interfaceByName(name); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-dhcp-iface",
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("DHCP server listen interface not found: ").Str(name).String(),
				Path:     "service/dhcp-server/listen-interface",
			})
		}
	}
	return diags
}
func forcedDoctorCode(code, configured string) bool {
	if configured == "" {
		return false
	}
	for item := range strings.SplitSeq(configured, ",") {
		if strings.TrimSpace(item) == code {
			return true
		}
	}
	return false
}
