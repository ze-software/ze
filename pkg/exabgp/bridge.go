// Package exabgp provides compatibility tools for ExaBGP plugins and configs.
package exabgp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Version is the ExaBGP version string used in the JSON envelope.
const Version = "5.0.0"

// ZebgpToExabgpJSON converts a ZeBGP JSON event to ExaBGP JSON format.
//
// ZeBGP format:
//
//	{
//	  "message": {"type": "update", "id": 1, "direction": "received"},
//	  "peer": {"address": "10.0.0.1", "asn": 65001},
//	  "origin": "igp",
//	  "ipv4/unicast": [{"action": "add", "next-hop": "10.0.0.1", "nlri": ["192.168.1.0/24"]}]
//	}
//
// ExaBGP format:
//
//	{
//	  "exabgp": "5.0.0",
//	  "type": "update",
//	  "neighbor": {
//	    "address": {"peer": "10.0.0.1"},
//	    "asn": {"peer": 65001},
//	    "direction": "receive",
//	    "message": {"update": {...}}
//	  }
//	}
func ZebgpToExabgpJSON(zebgp map[string]any) map[string]any {
	msg, _ := zebgp["message"].(map[string]any)
	msgType, _ := msg["type"].(string)
	if msgType == "" {
		msgType = "update"
	}

	peer, _ := zebgp["peer"].(map[string]any)
	peerAddr, _ := peer["address"].(string)
	peerASN, _ := peer["asn"].(float64)

	// Map direction: ZeBGP "received"/"sent" → ExaBGP "receive"/"send"
	direction, _ := msg["direction"].(string)
	switch direction {
	case "received":
		direction = "receive"
	case "sent":
		direction = "send"
	case "":
		direction = "receive"
	}

	// Build ExaBGP envelope
	result := map[string]any{
		"exabgp": Version,
		"time":   float64(time.Now().Unix()),
		"host":   hostname(),
		"pid":    os.Getpid(),
		"ppid":   os.Getppid(),
		"type":   msgType,
	}

	// Build neighbor section
	neighbor := map[string]any{
		"address":   map[string]any{"peer": peerAddr},
		"asn":       map[string]any{"peer": peerASN},
		"direction": direction,
	}

	switch msgType {
	case "state":
		state, _ := zebgp["state"].(string)
		neighbor["state"] = state

	case "update":
		update := convertUpdate(zebgp)
		if len(update) > 0 {
			neighbor["message"] = map[string]any{"update": update}
		}

	case "notification":
		notif, _ := zebgp["notification"].(map[string]any)
		neighbor["notification"] = map[string]any{
			"code":    notif["code"],
			"subcode": notif["subcode"],
			"data":    notif["data"],
		}
	}

	result["neighbor"] = neighbor
	return result
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func convertUpdate(zebgp map[string]any) map[string]any {
	update := make(map[string]any)

	// Extract attributes from top-level ZeBGP fields
	attrs := make(map[string]any)
	attrKeys := []string{
		"origin", "as-path", "med", "local-preference", "community",
		"large-community", "extended-community", "aggregator",
		"originator-id", "cluster-list", "atomic-aggregate",
	}
	for _, key := range attrKeys {
		if v, ok := zebgp[key]; ok {
			attrs[key] = v
		}
	}
	if len(attrs) > 0 {
		update["attribute"] = attrs
	}

	// Convert NLRI sections: "ipv4/unicast" → "ipv4 unicast"
	announce := make(map[string]map[string][]any)
	withdraw := make(map[string][]any)

	for key, value := range zebgp {
		if !strings.Contains(key, "/") || key == "as-path" {
			continue
		}

		// Convert family: "ipv4/unicast" → "ipv4 unicast"
		family := strings.ReplaceAll(key, "/", " ")

		entries, ok := value.([]any)
		if !ok {
			continue
		}

		for _, e := range entries {
			entry, ok := e.(map[string]any)
			if !ok {
				continue
			}

			action, _ := entry["action"].(string)
			nlriList, _ := entry["nlri"].([]any)
			nextHop, _ := entry["next-hop"].(string)

			switch action {
			case "add":
				nhKey := nextHop
				if nhKey == "" {
					nhKey = "null"
				}
				if announce[family] == nil {
					announce[family] = make(map[string][]any)
				}

				for _, nlri := range nlriList {
					if s, ok := nlri.(string); ok {
						announce[family][nhKey] = append(announce[family][nhKey], map[string]any{"nlri": s})
					} else {
						announce[family][nhKey] = append(announce[family][nhKey], nlri)
					}
				}
			case "del":
				for _, nlri := range nlriList {
					if s, ok := nlri.(string); ok {
						withdraw[family] = append(withdraw[family], map[string]any{"nlri": s})
					} else {
						withdraw[family] = append(withdraw[family], nlri)
					}
				}
			}
		}
	}

	if len(announce) > 0 {
		update["announce"] = announce
	}
	if len(withdraw) > 0 {
		update["withdraw"] = withdraw
	}

	return update
}

// ExabgpToZebgpCommand converts an ExaBGP text command to ZeBGP format.
//
// ExaBGP: neighbor <ip> announce route <prefix> next-hop <nh> [origin <o>] ...
// ZeBGP:  peer <ip> update text nhop set <nh> origin set <o> nlri ipv4/unicast add <prefix>.
func ExabgpToZebgpCommand(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}

	// Parse neighbor command
	neighborRE := regexp.MustCompile(`(?i)^neighbor\s+(\S+)\s+(.+)$`)
	match := neighborRE.FindStringSubmatch(line)
	if match == nil {
		// Not a neighbor command - pass through
		return line
	}

	peerIP := match[1]
	rest := strings.TrimSpace(match[2])
	restLower := strings.ToLower(rest)

	// Handle announce route
	if strings.HasPrefix(restLower, "announce route") {
		return convertAnnounce(peerIP, rest[14:])
	}

	// Handle withdraw route
	if strings.HasPrefix(restLower, "withdraw route") {
		return convertWithdraw(peerIP, rest[14:])
	}

	// Handle announce/withdraw for other families
	if strings.HasPrefix(restLower, "announce") {
		return convertAnnounceFamily(peerIP, rest[8:])
	}

	if strings.HasPrefix(restLower, "withdraw") {
		return convertWithdrawFamily(peerIP, rest[8:])
	}

	// Unknown command - pass through with peer prefix change
	return fmt.Sprintf("peer %s %s", peerIP, rest)
}

func convertAnnounce(peerIP, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		return fmt.Sprintf("peer %s update text nlri ipv4/unicast add", peerIP)
	}

	prefix := parts[0]
	attrs := parts[1:]

	// Parse attributes
	var cmdParts []string
	cmdParts = append(cmdParts, fmt.Sprintf("peer %s update text", peerIP))

	i := 0
	for i < len(attrs) {
		key := strings.ToLower(attrs[i])
		switch key {
		case "next-hop":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("nhop set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "origin":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("origin set %s", strings.ToLower(attrs[i+1])))
				i += 2
			} else {
				i++
			}
		case "as-path":
			if i+1 < len(attrs) {
				asp := attrs[i+1]
				i += 2
				if strings.HasPrefix(asp, "[") {
					// Collect until ]
					aspParts := []string{asp}
					for i < len(attrs) && !strings.Contains(aspParts[len(aspParts)-1], "]") {
						aspParts = append(aspParts, attrs[i])
						i++
					}
					asp = strings.Join(aspParts, " ")
				}
				asp = strings.Trim(asp, "[]")
				asp = strings.TrimSpace(asp)
				if asp != "" {
					cmdParts = append(cmdParts, fmt.Sprintf("as-path set %s", asp))
				}
			} else {
				i++
			}
		case "med":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("med set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "local-preference":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("local-preference set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "community":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("community add %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "large-community":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("large-community add %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		default:
			i++
		}
	}

	// Determine family from prefix
	family := "ipv4/unicast"
	if strings.Contains(prefix, ":") {
		family = "ipv6/unicast"
	}
	cmdParts = append(cmdParts, fmt.Sprintf("nlri %s add %s", family, prefix))

	return strings.Join(cmdParts, " ")
}

func convertWithdraw(peerIP, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		return fmt.Sprintf("peer %s update text nlri ipv4/unicast del", peerIP)
	}

	prefix := parts[0]
	family := "ipv4/unicast"
	if strings.Contains(prefix, ":") {
		family = "ipv6/unicast"
	}
	return fmt.Sprintf("peer %s update text nlri %s del %s", peerIP, family, prefix)
}

func convertAnnounceFamily(peerIP, rest string) string {
	rest = strings.TrimSpace(rest)
	familyRE := regexp.MustCompile(`(?i)^(ipv[46])\s+(unicast|multicast|nlri-mpls|flowspec)\s+(.+)$`)
	match := familyRE.FindStringSubmatch(rest)
	if match != nil {
		afi := strings.ToLower(match[1])
		safi := strings.ToLower(match[2])
		routeStr := match[3]
		family := fmt.Sprintf("%s/%s", afi, safi)
		return convertAnnounceWithFamily(peerIP, family, routeStr)
	}

	// Fall back to basic conversion
	return fmt.Sprintf("peer %s announce %s", peerIP, rest)
}

func convertWithdrawFamily(peerIP, rest string) string {
	rest = strings.TrimSpace(rest)
	familyRE := regexp.MustCompile(`(?i)^(ipv[46])\s+(unicast|multicast|nlri-mpls|flowspec)\s+(.+)$`)
	match := familyRE.FindStringSubmatch(rest)
	if match != nil {
		afi := strings.ToLower(match[1])
		safi := strings.ToLower(match[2])
		prefix := strings.Fields(match[3])[0]
		family := fmt.Sprintf("%s/%s", afi, safi)
		return fmt.Sprintf("peer %s update text nlri %s del %s", peerIP, family, prefix)
	}

	return fmt.Sprintf("peer %s withdraw %s", peerIP, rest)
}

func convertAnnounceWithFamily(peerIP, family, routeStr string) string {
	routeStr = strings.TrimSpace(routeStr)
	parts := strings.Fields(routeStr)
	if len(parts) == 0 {
		return fmt.Sprintf("peer %s update text nlri %s add", peerIP, family)
	}

	prefix := parts[0]
	attrs := parts[1:]

	var cmdParts []string
	cmdParts = append(cmdParts, fmt.Sprintf("peer %s update text", peerIP))

	i := 0
	for i < len(attrs) {
		key := strings.ToLower(attrs[i])
		switch key {
		case "next-hop":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("nhop set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "origin":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("origin set %s", strings.ToLower(attrs[i+1])))
				i += 2
			} else {
				i++
			}
		case "label":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("label set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		case "rd":
			if i+1 < len(attrs) {
				cmdParts = append(cmdParts, fmt.Sprintf("rd set %s", attrs[i+1]))
				i += 2
			} else {
				i++
			}
		default:
			i++
		}
	}

	cmdParts = append(cmdParts, fmt.Sprintf("nlri %s add %s", family, prefix))
	return strings.Join(cmdParts, " ")
}

// Bridge wraps an ExaBGP plugin process and translates between ZeBGP and ExaBGP formats.
type Bridge struct {
	pluginCmd []string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	running   bool
	mu        sync.Mutex
}

// NewBridge creates a new bridge for the given ExaBGP plugin command.
func NewBridge(pluginCmd []string) *Bridge {
	return &Bridge{
		pluginCmd: pluginCmd,
	}
}

// Start starts the plugin subprocess with the given context.
func (b *Bridge) Start(ctx context.Context) error {
	var err error

	//nolint:gosec // User-provided plugin command is intentional.
	b.cmd = exec.CommandContext(ctx, b.pluginCmd[0], b.pluginCmd[1:]...)

	b.stdin, err = b.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	b.stdout, err = b.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	b.stderr, err = b.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := b.cmd.Start(); err != nil {
		return fmt.Errorf("start plugin: %w", err)
	}

	b.running = true
	return nil
}

// Stop stops the plugin subprocess.
func (b *Bridge) Stop() {
	b.mu.Lock()
	b.running = false
	b.mu.Unlock()

	if b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
	}
}

// Run runs the bridge, translating between ZeBGP (stdin/stdout) and the plugin.
func (b *Bridge) Run(ctx context.Context) error {
	if err := b.Start(ctx); err != nil {
		return err
	}
	defer b.Stop()

	var wg sync.WaitGroup
	wg.Add(3)

	// ZeBGP stdin → plugin stdin (translate ZeBGP JSON → ExaBGP JSON)
	go func() {
		defer wg.Done()
		b.zebgpToPlugin(ctx, os.Stdin, b.stdin)
	}()

	// Plugin stdout → ZeBGP stdout (translate ExaBGP commands → ZeBGP commands)
	go func() {
		defer wg.Done()
		b.pluginToZebgp(ctx, b.stdout, os.Stdout)
	}()

	// Plugin stderr → ZeBGP stderr (pass through)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stderr, b.stderr)
	}()

	// Wait for plugin to exit
	err := b.cmd.Wait()
	wg.Wait()
	return err
}

func (b *Bridge) zebgpToPlugin(ctx context.Context, r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		b.mu.Lock()
		running := b.running
		b.mu.Unlock()
		if !running {
			return
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var zebgp map[string]any
		if err := json.Unmarshal([]byte(line), &zebgp); err != nil {
			continue
		}

		exabgp := ZebgpToExabgpJSON(zebgp)
		out, err := json.Marshal(exabgp)
		if err != nil {
			continue
		}

		_, _ = fmt.Fprintln(w, string(out))
	}
}

func (b *Bridge) pluginToZebgp(ctx context.Context, r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		b.mu.Lock()
		running := b.running
		b.mu.Unlock()
		if !running {
			return
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		zebgpCmd := ExabgpToZebgpCommand(line)
		if zebgpCmd != "" {
			_, _ = fmt.Fprintln(w, zebgpCmd)
		}
	}
}
