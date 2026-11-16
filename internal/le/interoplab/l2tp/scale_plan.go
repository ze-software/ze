// Design: docs/labs/l2tp-interop.md -- the many-sessions LNS topology.
// Related: register_scale.go -- scenario and image registration.
package l2tp

import (
	"context"
	"net/netip"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scaleFileProbeCommand = "test"
)

func scaleScenarioPlan(root string, source interoplab.ScenarioSource, environment interoplab.Environment) interoplab.ScenarioPlan {
	peers := scalePeerTemplates(root, source.Directory, environment.Suffix)
	plan := interoplab.ScenarioPlan{
		Source: source,
		Network: interoplab.NetworkSpec{
			Name:       "ze-l2tp-" + environment.Suffix,
			Candidates: []interoplab.Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}},
		},
		Peers: peers,
	}
	for index := range peers {
		plan.Containers = append(plan.Containers, peers[index].Container)
	}
	plan.Prepare = func(_ context.Context, prepared interoplab.PrepareContext) (interoplab.PreparedScenario, error) {
		ready := scalePeerTemplates(root, source.Directory, environment.Suffix)
		ze, err := addressAtHost(prepared.Network.IPv4, 2)
		if err != nil {
			return interoplab.PreparedScenario{}, err
		}
		for index := range scaleLACCount {
			// The fixed topology has three infrastructure peers, then one
			// relay and LAC pair per independently addressed tunnel.
			relayIndex := 3 + index*2
			lac, err := addressAtHost(prepared.Network.IPv4, ready[relayIndex+1].Host)
			if err != nil {
				return interoplab.PreparedScenario{}, err
			}
			ready[relayIndex].Command = []string{
				netip.AddrPortFrom(ze, 1701).String(), netip.AddrPortFrom(lac, 1702).String(), "scale",
			}
		}
		return interoplab.PreparedScenario{Peers: ready}, nil
	}
	return plan
}

// scalePeerTemplates is the one declaration of scale participants. Preparation,
// image selection, diagnostics and cleanup all derive their population here.
func scalePeerTemplates(root, directory, suffix string) []interoplab.PeerConfig {
	ze := zePeer(suffix, filepath.Join(directory, "ze.conf"))
	ze.Image = imageZeScale
	peers := make([]interoplab.PeerConfig, 0, 3+2*scaleLACCount)
	peers = append(peers, scaleRadiusPeer(directory, suffix), frrPeer(root, suffix, directory), ze)
	for index := range scaleLACCount {
		number := strconv.Itoa(index + 1)
		relay := interoplab.PeerConfig{
			Name: "scale-relay-" + number, Container: "ze-l2tp-scale-relay-" + number + "-" + suffix,
			Image: imageAVPRelay, Host: uint8(21 + index),
			Ready: &interoplab.ReadyProbe{
				Command: []string{scaleFileProbeCommand, "-f", "/run/ready"}, Timeout: 15 * time.Second, Interval: time.Second,
			},
		}
		lac := interoplab.PeerConfig{
			Name: "kl2tpd-" + number, Container: "ze-l2tp-kl2tpd-" + number + "-" + suffix,
			Image: imageKL2TP, Host: uint8(11 + index),
			Mounts: []interoplab.Mount{
				{Source: filepath.Join(directory, "lac-"+number+".toml"), Target: "/etc/kl2tpd/kl2tpd.toml", ReadOnly: true},
			},
			Capabilities: []string{netAdminCapability}, Arguments: interoplab.PPPDeviceArguments(),
			// The checker MUST launch each configured LAC only when it can
			// measure that tunnel's short real CWND4 hold. Suite cleanup MUST
			// stop this container and its detached kl2tpd/pppd children.
			Command: []string{"sleep", "infinity"},
			Ready: &interoplab.ReadyProbe{
				Command: []string{scaleFileProbeCommand, "-r", "/etc/kl2tpd/kl2tpd.toml"}, Timeout: 15 * time.Second, Interval: time.Second,
			},
		}
		for session := 1; session <= scaleSessionsPerLAC; session++ {
			user := scaleUsername(index+1, session)
			lac.Mounts = append(lac.Mounts, interoplab.Mount{
				Source: filepath.Join(directory, "ppp", user), Target: "/etc/ppp/scale/" + user, ReadOnly: true,
			})
		}
		peers = append(peers, relay, lac)
	}
	return peers
}

func scaleRadiusPeer(directory, suffix string) interoplab.PeerConfig {
	return interoplab.PeerConfig{
		Name: peerRadius, Container: "ze-l2tp-radius-" + suffix,
		Image: interoplab.FreeRADIUSImageName, Host: 5,
		Mounts: []interoplab.Mount{
			{Source: filepath.Join(directory, "clients.conf"), Target: "/etc/raddb/clients.conf", ReadOnly: true},
			{Source: filepath.Join(directory, "site-default"), Target: "/etc/raddb/sites-enabled/default", ReadOnly: true},
			{Source: filepath.Join(directory, "mods-ze-detail"), Target: "/etc/raddb/mods-enabled/ze_detail", ReadOnly: true},
			{Source: filepath.Join(directory, "users"), Target: "/etc/raddb/mods-config/files/authorize", ReadOnly: true},
		},
		Command: []string{"radiusd", "-f", "-X"},
		Ready: &interoplab.ReadyProbe{
			Command: []string{"sh", "-c", "grep -q ':0714 ' /proc/net/udp && grep -q ':0715 ' /proc/net/udp"},
			Timeout: 90 * time.Second, Interval: time.Second,
		},
	}
}

func scaleImageBuilds(root string, source interoplab.ScenarioSource, frrImage string) []interoplab.ImageBuild {
	// The derived images reuse the maintained Ze runtime and pppd LAC build.
	names := []string{imageZe, imageLAC}
	peers := scalePeerTemplates(root, source.Directory, "")
	for index := range peers {
		if peers[index].Image != imageFRR {
			names = append(names, peers[index].Image)
		}
	}
	builds, _, err := interoplab.ResolveImages(interoplab.ImageContext{Root: root}, names)
	if err != nil {
		// These names are compile-time declarations, never external input.
		panic("BUG: scale scenario declares an unresolved image: " + err.Error())
	}
	return append(builds, interoplab.ImageBuild{Name: imageFRR, Tag: frrImage, Pull: true, Required: true})
}
