// Design: docs/architecture/plugin/rib-storage-design.md — RPKI origin validation plugin
// Detail: rtr_pdu.go — RTR PDU wire format types and parsing
// Detail: rtr_session.go — RTR session lifecycle management
// Detail: roa_cache.go — ROA cache VRP storage and covering-prefix lookup
// Detail: validate.go — RFC 6811 origin validation algorithm
package rpki

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	bgp "codeberg.org/thomas-mangin/ze/internal/component/bgp"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

const (
	statusDone  = "done"
	statusError = "error"
)

// RPKIPlugin implements the bgp-rpki plugin.
// It manages RTR sessions to RPKI cache servers, maintains the ROA cache,
// and validates received routes against VRPs.
type RPKIPlugin struct {
	plugin *sdk.Plugin
	cache  *ROACache
	mu     sync.RWMutex

	// sessions holds active RTR sessions to cache servers.
	sessions []*RTRSession

	// stopCh signals all background goroutines to stop.
	stopCh chan struct{}
}

// RunRPKIPlugin runs the bgp-rpki plugin using the SDK RPC protocol.
func RunRPKIPlugin(engineConn, callbackConn net.Conn) int {
	logger().Debug("bgp-rpki plugin starting")

	p := sdk.NewWithConn("bgp-rpki", engineConn, callbackConn)
	defer func() { _ = p.Close() }()

	rp := &RPKIPlugin{
		plugin: p,
		cache:  NewROACache(),
		stopCh: make(chan struct{}),
	}
	defer close(rp.stopCh)

	p.OnEvent(func(jsonStr string) error {
		event, err := bgp.ParseEvent([]byte(jsonStr))
		if err != nil {
			logger().Warn("rpki: parse error", "error", err, "line", jsonStr[:min(100, len(jsonStr))])
			return nil
		}
		rp.handleEvent(event)
		return nil
	})

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
		return rp.handleCommand(command, strings.Join(args, " "))
	})

	// Enable validation gate in adj-rib-in after plugin startup completes.
	p.OnStarted(func(startCtx context.Context) error {
		enableCtx, cancel := context.WithTimeout(startCtx, 10*time.Second)
		defer cancel()
		status, _, err := p.DispatchCommand(enableCtx, "adj-rib-in enable-validation")
		if err != nil {
			logger().Error("rpki: failed to enable validation gate", "error", err)
			return err
		}
		logger().Info("rpki: validation gate enabled", "status", status)
		return nil
	})

	p.SetStartupSubscriptions([]string{"update direction received"}, nil, "full")

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "rpki status"},
			{Name: "rpki cache"},
			{Name: "rpki roa"},
			{Name: "rpki summary"},
		},
	})
	if err != nil {
		logger().Error("bgp-rpki plugin failed", "error", err)
		return 1
	}

	return 0
}

// handleEvent processes BGP events (UPDATE received).
func (rp *RPKIPlugin) handleEvent(event *bgp.Event) {
	eventType := event.GetEventType()
	if eventType != "update" {
		return
	}

	peerAddr := event.GetPeerAddress()
	if peerAddr == "" {
		return
	}

	// Validate each NLRI prefix against the ROA cache.
	for family, ops := range event.FamilyOps {
		for _, op := range ops {
			if op.Action != "add" {
				continue
			}
			// Extract origin AS from AS_PATH in raw attributes.
			originAS := extractOriginAS(event.RawAttributes)

			for _, nlriVal := range op.NLRIs {
				prefix, _ := bgp.ParseNLRIValue(nlriVal)
				if prefix == "" {
					continue
				}

				state := rp.cache.Validate(prefix, originAS)
				rp.issueValidationDecision(peerAddr, family, prefix, state)
			}
		}
	}
}

// issueValidationDecision sends accept-routes or reject-routes to adj-rib-in.
func (rp *RPKIPlugin) issueValidationDecision(peerAddr, family, prefix string, state uint8) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if state == ValidationInvalid {
		cmd := "adj-rib-in reject-routes " + peerAddr + " " + family + " " + prefix
		_, _, err := rp.plugin.DispatchCommand(ctx, cmd)
		if err != nil {
			logger().Warn("rpki: reject-routes failed", "error", err, "prefix", prefix)
		}
	} else {
		// Valid or NotFound -- accept with the validation state.
		stateStr := "2" // NotFound
		if state == ValidationValid {
			stateStr = "1"
		}
		cmd := "adj-rib-in accept-routes " + peerAddr + " " + family + " " + prefix + " " + stateStr
		_, _, err := rp.plugin.DispatchCommand(ctx, cmd)
		if err != nil {
			logger().Warn("rpki: accept-routes failed", "error", err, "prefix", prefix)
		}
	}
}

// handleCommand processes RPKI CLI commands.
func (rp *RPKIPlugin) handleCommand(command, _ string) (string, string, error) {
	switch command {
	case "rpki status":
		return rp.statusCommand()
	case "rpki cache":
		return rp.cacheCommand()
	case "rpki roa":
		return rp.roaCommand()
	case "rpki summary":
		return rp.summaryCommand()
	}
	return statusError, "", fmt.Errorf("unknown command: %s", command)
}

func (rp *RPKIPlugin) statusCommand() (string, string, error) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	v4, v6 := rp.cache.Count()
	data := fmt.Sprintf(`{"running":true,"vrp-count-ipv4":%d,"vrp-count-ipv6":%d,"sessions":%d}`,
		v4, v6, len(rp.sessions))
	return statusDone, data, nil
}

func (rp *RPKIPlugin) cacheCommand() (string, string, error) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	data := `{"cache-servers":[]}`
	return statusDone, data, nil
}

func (rp *RPKIPlugin) roaCommand() (string, string, error) {
	v4, v6 := rp.cache.Count()
	data := fmt.Sprintf(`{"total-vrps":%d,"ipv4-vrps":%d,"ipv6-vrps":%d}`, v4+v6, v4, v6)
	return statusDone, data, nil
}

func (rp *RPKIPlugin) summaryCommand() (string, string, error) {
	v4, v6 := rp.cache.Count()
	data := fmt.Sprintf(`{"vrp-count":%d,"validation-enabled":true}`, v4+v6)
	return statusDone, data, nil
}
