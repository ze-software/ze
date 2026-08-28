package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func p12AS112Announce(ctx context.Context, plugin *sdk.Plugin) error {
	peers := []p12Peer{{"peer4", "127.0.0.1"}, {"peer6", "::1"}}
	if err := p12WaitAllPeerFields(ctx, plugin, peers, "state", func(_ p12Peer, value any) bool {
		return value == "established"
	}, "both peers established"); err != nil {
		return err
	}
	if err := p12WaitAllPeerFields(ctx, plugin, peers, "eor-sent", func(_ p12Peer, value any) bool {
		return p12NumberAtLeast(value, 1)
	}, "initial EOR sent on both sessions"); err != nil {
		return err
	}
	base := make(map[string]int, len(peers))
	for _, peer := range peers {
		value, ok := p12Number(p12PeerField(ctx, plugin, peer.name, peer.addr, "updates-sent"))
		if !ok {
			return fmt.Errorf("%s updates-sent is not an integer", peer.name)
		}
		base[peer.name] = value
	}
	if _, err := p12RequireDone(ctx, plugin, "request fakeas112 emit add asn 112"); err != nil {
		return err
	}
	if err := p12WaitAllPeerFields(ctx, plugin, peers, "updates-sent", func(peer p12Peer, value any) bool {
		return p12NumberAtLeast(value, base[peer.name]+1)
	}, "covering-prefix UPDATEs on both wires"); err != nil {
		return err
	}
	return nil
}

func p12AS112Single(command string) p12Scenario {
	return func(ctx context.Context, plugin *sdk.Plugin) error {
		peer := p12Peer{"peer1", "127.0.0.1"}
		if err := p12WaitPeerField(ctx, plugin, peer, "state", func(value any) bool { return value == "established" }, "peer1 established"); err != nil {
			return err
		}
		if err := p12WaitPeerField(ctx, plugin, peer, "eor-sent", func(value any) bool { return p12NumberAtLeast(value, 1) }, "initial EOR sent"); err != nil {
			return err
		}
		base, ok := p12Number(p12PeerField(ctx, plugin, peer.name, peer.addr, "updates-sent"))
		if !ok {
			return fmt.Errorf("peer1 updates-sent is not an integer")
		}
		if _, err := p12RequireDone(ctx, plugin, command); err != nil {
			return err
		}
		return p12WaitPeerField(ctx, plugin, peer, "updates-sent", func(value any) bool { return p12NumberAtLeast(value, base+1) }, "covering-prefix UPDATE sent")
	}
}

func p12AS112Withdraw(ctx context.Context, plugin *sdk.Plugin) error {
	peer := p12Peer{"peer1", "127.0.0.1"}
	if err := p12WaitPeerField(ctx, plugin, peer, "state", func(value any) bool { return value == "established" }, "peer1 established"); err != nil {
		return err
	}
	if err := p12WaitPeerField(ctx, plugin, peer, "eor-sent", func(value any) bool { return p12NumberAtLeast(value, 1) }, "initial EOR sent"); err != nil {
		return err
	}
	base, ok := p12Number(p12PeerField(ctx, plugin, peer.name, peer.addr, "updates-sent"))
	if !ok {
		return fmt.Errorf("peer1 updates-sent is not an integer")
	}
	for index, command := range []string{"request fakeas112 emit add asn 112", "request fakeas112 emit del asn 112"} {
		if _, err := p12RequireDone(ctx, plugin, command); err != nil {
			return err
		}
		minimum := base + index + 1
		if err := p12WaitPeerField(ctx, plugin, peer, "updates-sent", func(value any) bool { return p12NumberAtLeast(value, minimum) }, fmt.Sprintf("UPDATE %d on the wire", index+1)); err != nil {
			return err
		}
	}
	return nil
}

func p12RedistributeNotConfigured(source string) p12Scenario {
	return func(ctx context.Context, plugin *sdk.Plugin) error {
		peer := p12Peer{"peer1", "127.0.0.1"}
		if err := p12WaitPeerField(ctx, plugin, peer, "connections-established", func(value any) bool { return p12NumberAtLeast(value, 1) }, "peer1 session established at least once"); err != nil {
			return err
		}
		command := "request " + source + " emit add "
		if source == "fakeas112" {
			command += "asn 112"
		} else {
			command += "ipv4/unicast 10.0.0.1/32"
		}
		if _, err := p12RequireDone(ctx, plugin, command); err != nil {
			return err
		}
		p12Settle(ctx, 10, 200*time.Millisecond)
		return nil
	}
}

func p12L2TPAnnounce(ctx context.Context, plugin *sdk.Plugin) error {
	p12Settle(ctx, 10, 200*time.Millisecond)
	if _, err := p12RequireDone(ctx, plugin, "request fakel2tp emit add ipv4/unicast 10.0.0.1/32"); err != nil {
		return err
	}
	p12Settle(ctx, 10, 200*time.Millisecond)
	return nil
}

func p12L2TPMultiPeer(ctx context.Context, plugin *sdk.Plugin) error {
	p12Settle(ctx, 10, 200*time.Millisecond)
	if _, err := p12RequireDone(ctx, plugin, "request fakel2tp emit add ipv4/unicast 10.0.0.1/32"); err != nil {
		return err
	}
	p12Settle(ctx, 15, 200*time.Millisecond)
	return nil
}

func p12L2TPWithdraw(ctx context.Context, plugin *sdk.Plugin) error {
	p12Settle(ctx, 10, 200*time.Millisecond)
	for _, command := range []string{
		"request fakel2tp emit add ipv4/unicast 10.0.0.1/32",
		"request fakel2tp emit remove ipv4/unicast 10.0.0.1/32",
	} {
		if _, err := p12RequireDone(ctx, plugin, command); err != nil {
			return err
		}
		p12Settle(ctx, 3, 200*time.Millisecond)
	}
	p12Settle(ctx, 10, 200*time.Millisecond)
	return nil
}

func p12LateJoinConfigAdd(ctx context.Context, plugin *sdk.Plugin) error {
	_, err := p12RequireDone(ctx, plugin, "request fakeredist emit add ipv4/unicast 10.0.0.1/32")
	return err
}

func p12LateJoinTrigger(ctx context.Context, _ []string) error {
	if err := p12WaitForFile(ctx, "daemon.pid"); err != nil {
		return err
	}
	timer := time.NewTimer(6 * time.Second)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
	}
	if err := p12CopyFile("config2.conf", "ze-bgp.conf"); err != nil {
		return err
	}
	return p12SignalDaemon("daemon.pid")
}

func p12RelayWithdraw(reflector bool) p12Scenario {
	return func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := p12Quiesce(ctx, plugin); err != nil {
			return err
		}
		destUpdates := func() (int, bool) {
			updates, updatesOK := p12PeerCounter(ctx, plugin, "127.0.0.2", "updates-sent")
			eors, eorsOK := p12PeerCounter(ctx, plugin, "127.0.0.2", "eor-sent")
			return updates - eors, updatesOK && eorsOK
		}
		if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
			updates, ok := destUpdates()
			return ok && updates >= 1
		}) {
			if reflector {
				return fmt.Errorf("dest peer was never sent the reflected advertisement")
			}
			return fmt.Errorf("dest peer was never sent the advertisement")
		}
		if reflector {
			fmt.Fprintln(os.Stderr, "OK: dest was sent the reflected advertisement")
		} else {
			fmt.Fprintln(os.Stderr, "OK: dest was sent the advertisement")
		}

		if _, _, err := plugin.UpdateRoute(ctx, "127.0.0.1", "update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 192.0.2.0/24"); err != nil {
			return err
		}
		if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
			updates, ok := destUpdates()
			return ok && updates >= 2
		}) {
			if reflector {
				return fmt.Errorf("dest peer was never sent the reflected withdrawal")
			}
			return fmt.Errorf("dest peer was never sent the forwarded withdrawal")
		}
		if reflector {
			fmt.Fprintln(os.Stderr, "OK: dest was sent the reflected withdrawal")
		} else {
			fmt.Fprintln(os.Stderr, "OK: dest was sent the forwarded withdrawal")
		}
		return nil
	}
}
