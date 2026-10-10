// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: register_kernelcap_linux.go -- the init() that enrols what this file builds
// Related: xfrm_linux.go -- the algorithm tables the cipher enrolment is derived from
// Related: xfrm_migrate_linux.go -- the MOBIKE migration probe
// Related: internal/component/kernelcap -- the enrolment these capabilities join
//
// The XFRM backend needs more of the kernel than the XFRM netlink socket the
// engine enrols: ESP for each address family, every transform its algorithm
// tables can name, and XFRM_MSG_MIGRATE_STATE for MOBIKE. Each is a capability
// here, so ze doctor reports a missing one and a Docker host that lacks one is
// refused before a lab spends minutes failing on it.
//
// The ESP and transform probes send XFRM_MSG_UPDSA for an SA that cannot exist.
// The kernel builds the whole state first (xfrm_state_construct: algorithm
// lookup, then the ESP type and its crypto transform with the key) and only then
// looks for the SA to replace, so ESRCH proves every piece was found while
// nothing was installed or changed.

//go:build linux

package dataplane

import (
	"errors"
	"net"
	"slices"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const kernelcapComponent = "ike"

// RFC 4303 Section 2.1: "The set of SPI values in the range 1 through 255 are
// reserved by the Internet Assigned Numbers Authority (IANA) for future use".
// No negotiated or manual SA carries one, so an update naming it finds no SA.
const xfrmProbeSPI = 1

// The probe SA's endpoints are documentation addresses (RFC 5737 TEST-NET-1,
// RFC 3849), which no tunnel ever uses, so the update cannot find a real SA even
// on a host that broke the SPI reservation above.
var (
	xfrmProbeSource4 = net.IPv4(192, 0, 2, 2)
	xfrmProbeTarget4 = net.IPv4(192, 0, 2, 1)
	xfrmProbeSource6 = net.ParseIP("2001:db8::2")
	xfrmProbeTarget6 = net.ParseIP("2001:db8::1")
)

// xfrmProbeUpdate sends the XFRM_MSG_UPDSA. Tests replace it and MUST restore it.
var xfrmProbeUpdate = netlink.XfrmStateUpdate

// xfrmTransformKernel is what the cipher enrolment needs to know about one kernel
// transform name that xfrm_linux.go's tables hold: the kernel build symbol that
// provides it and a key length the kernel's setkey accepts for it. It is keyed by
// the transform, never by the algorithm word, and xfrmTransformCapabilities
// refuses at init a transform this table does not describe, so a word added to
// those tables alone fails every start rather than going unrequired.
var xfrmTransformKernel = map[string]struct {
	kernel    string
	keyOctets int
}{
	xfrmEncAESCBC:                   {kernel: "CONFIG_CRYPTO_CBC", keyOctets: 16},
	"cbc(des3_ede)":                 {kernel: "CONFIG_CRYPTO_DES", keyOctets: 24},
	"ecb(cipher_null)":              {kernel: "CONFIG_CRYPTO_NULL", keyOctets: 0},
	xfrmAEADAESGCM:                  {kernel: "CONFIG_CRYPTO_GCM", keyOctets: 20},
	"rfc7539esp(chacha20,poly1305)": {kernel: "CONFIG_CRYPTO_CHACHA20POLY1305", keyOctets: 36},
	xfrmAuthSHA256:                  {kernel: "CONFIG_CRYPTO_SHA256", keyOctets: 32},
	"hmac(sha384)":                  {kernel: "CONFIG_CRYPTO_SHA512", keyOctets: 48},
	"hmac(sha512)":                  {kernel: "CONFIG_CRYPTO_SHA512", keyOctets: 64},
	"hmac(sha1)":                    {kernel: "CONFIG_CRYPTO_SHA1", keyOctets: 20},
}

// xfrmTransformRole says where a transform goes in an SA, which decides the
// partner the probe pairs it with.
type xfrmTransformRole uint8

const (
	xfrmRoleUnspecified xfrmTransformRole = iota
	xfrmRoleCipher
	xfrmRoleAEAD
	xfrmRoleIntegrity
)

// xfrmKernelCapabilities returns every capability the XFRM backend enrols: the
// MOBIKE migration message, ESP for each family, and one per kernel transform
// the algorithm tables name. The transforms are DERIVED from those tables, so a
// cipher the backend learns to install is required of the host with no edit here.
func xfrmKernelCapabilities() []kernelcap.Capability {
	capabilities := []kernelcap.Capability{
		{
			Subsystem:   "ipsec-mobike",
			Component:   kernelcapComponent,
			Kernel:      "CONFIG_XFRM_MIGRATE",
			ConfigLeaf:  "vpn ipsec",
			Degrades:    "MOBIKE is not offered, so a peer whose address changes must renegotiate its IKE SA",
			CodeAbsent:  diagnostic.CodeDoctorIPsecMOBIKEUnavailable,
			CodeUnknown: diagnostic.CodeDoctorIPsecMOBIKEUnknown,
			Order:       736,
			InUse:       kernelcap.IPsecInUse,
			Probe:       xfrmMigrationProbe,
		},
		xfrmESPCapability("ipsec-esp-ipv4", "CONFIG_INET_ESP", "an IPv4 Child SA fails to install", xfrmProbeSource4, xfrmProbeTarget4),
		xfrmESPCapability("ipsec-esp-ipv6", "CONFIG_INET6_ESP", "an IPv6 Child SA fails to install", xfrmProbeSource6, xfrmProbeTarget6),
	}
	return append(capabilities, xfrmTransformCapabilities()...)
}

func xfrmESPCapability(subsystem, kernel, degrades string, source, target net.IP) kernelcap.Capability {
	return kernelcap.Capability{
		Subsystem:   subsystem,
		Component:   kernelcapComponent,
		Kernel:      kernel,
		ConfigLeaf:  "vpn ipsec",
		Degrades:    degrades,
		CodeAbsent:  diagnostic.CodeDoctorIPsecESPUnavailable,
		CodeUnknown: diagnostic.CodeDoctorIPsecESPUnknown,
		Order:       737,
		InUse:       kernelcap.IPsecInUse,
		Probe: func() kernelcap.Result {
			return classifyESPProbe(xfrmProbeUpdate(xfrmProbeState(source, target, xfrmRoleAEAD, xfrmAEADAESGCM)))
		},
	}
}

// xfrmTransformCapabilities walks the three algorithm tables and enrols each
// distinct kernel transform once, naming every algorithm word that maps to it.
func xfrmTransformCapabilities() []kernelcap.Capability {
	words := make(map[string][]string)
	roles := make(map[string]xfrmTransformRole)
	for word, transform := range xfrmEncNames {
		words[transform] = append(words[transform], word)
		roles[transform] = xfrmRoleCipher
	}
	for word, transform := range xfrmAEADNames {
		words[transform] = append(words[transform], word)
		roles[transform] = xfrmRoleAEAD
	}
	for word, auth := range xfrmAuthNames {
		words[auth.name] = append(words[auth.name], word)
		roles[auth.name] = xfrmRoleIntegrity
	}

	transforms := make([]string, 0, len(words))
	for transform := range words {
		transforms = append(transforms, transform)
	}
	slices.Sort(transforms)

	capabilities := make([]kernelcap.Capability, 0, len(transforms))
	for _, transform := range transforms {
		facts, known := xfrmTransformKernel[transform]
		if !known {
			panic("BUG: kernelcap: xfrm transform " + transform + " has no kernel symbol in xfrmTransformKernel")
		}
		slices.Sort(words[transform])
		capabilities = append(capabilities, xfrmTransformCapability(transform, facts.kernel, roles[transform], words[transform]))
	}
	return capabilities
}

func xfrmTransformCapability(transform, kernel string, role xfrmTransformRole, words []string) kernelcap.Capability {
	var leaf textbuf.Buffer
	leaf.Str("an ESP proposal naming ").Str(strings.Join(words, " or "))
	var loss textbuf.Buffer
	loss.Str("a Child SA negotiated with ").Str(strings.Join(words, " or ")).
		Str(" fails to install, because the kernel cannot run ").Str(transform)

	return kernelcap.Capability{
		Subsystem:   "ipsec-transform-" + kernelSubsystemWord(transform),
		Component:   kernelcapComponent,
		Kernel:      kernel,
		ConfigLeaf:  leaf.String(),
		Degrades:    loss.String(),
		CodeAbsent:  diagnostic.CodeDoctorIPsecTransformUnavailable,
		CodeUnknown: diagnostic.CodeDoctorIPsecTransformUnknown,
		Order:       738,
		InUse:       kernelcap.IPsecInUse,
		Probe: func() kernelcap.Result {
			return classifyTransformProbe(xfrmProbeUpdate(xfrmProbeState(xfrmProbeSource4, xfrmProbeTarget4, role, transform)))
		},
	}
}

// kernelSubsystemWord spells a kernel transform name in lower-kebab:
// "rfc4106(gcm(aes))" becomes "rfc4106-gcm-aes".
func kernelSubsystemWord(transform string) string {
	fields := strings.FieldsFunc(transform, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	return strings.Join(fields, "-")
}

// xfrmProbeState builds the state the probe asks the kernel to replace. The
// transform under test takes its role; a cipher is paired with HMAC-SHA-256 and an
// integrity transform with AES-CBC, Ze's own defaults, because ESP needs both.
func xfrmProbeState(source, target net.IP, role xfrmTransformRole, transform string) *netlink.XfrmState {
	state := &netlink.XfrmState{
		Src:   source,
		Dst:   target,
		Proto: netlink.XFRM_PROTO_ESP,
		Mode:  netlink.XFRM_MODE_TRANSPORT,
		Spi:   xfrmProbeSPI,
	}
	cipher := &netlink.XfrmStateAlgo{Name: xfrmEncAESCBC, Key: xfrmProbeKey(xfrmEncAESCBC)}
	integrity := &netlink.XfrmStateAlgo{Name: xfrmAuthSHA256, Key: xfrmProbeKey(xfrmAuthSHA256), TruncateLen: 128}
	switch role {
	case xfrmRoleAEAD:
		// Every AEAD the backend installs carries a 16-octet ICV (xfrmAEADNames).
		state.Aead = &netlink.XfrmStateAlgo{Name: transform, Key: xfrmProbeKey(transform), ICVLen: 128}
		return state
	case xfrmRoleCipher:
		cipher = &netlink.XfrmStateAlgo{Name: transform, Key: xfrmProbeKey(transform)}
	case xfrmRoleIntegrity:
		integrity = &netlink.XfrmStateAlgo{Name: transform, Key: xfrmProbeKey(transform), TruncateLen: xfrmProbeTruncation(transform)}
	case xfrmRoleUnspecified:
		panic("BUG: kernelcap: xfrm probe built with no transform role")
	}
	state.Crypt = cipher
	state.Auth = integrity
	return state
}

// xfrmProbeKey returns a zero key of the length the kernel accepts for transform.
// The probe SA is never installed, so the key material is never used.
func xfrmProbeKey(transform string) []byte {
	return make([]byte, xfrmTransformKernel[transform].keyOctets)
}

// xfrmProbeTruncation returns the ICV length the backend installs transform with.
func xfrmProbeTruncation(transform string) int {
	for _, auth := range xfrmAuthNames {
		if auth.name == transform {
			return auth.truncLen
		}
	}
	panic("BUG: kernelcap: integrity transform " + transform + " is not in xfrmAuthNames")
}

// errProbeUpdateAccepted is reported when the kernel accepted the update of an SA
// that cannot exist. It proves nothing about the feature, so it is no verdict.
var errProbeUpdateAccepted = errors.New("the kernel accepted an update of an SA that cannot exist")

// classifyESPProbe reads the update's errno for the ESP capability. ESRCH comes
// after the kernel built the ESP state, so it is presence. EPROTONOSUPPORT is
// the kernel holding no ESP type for the family (xfrm_get_type), or no XFRM
// netlink at all. Anything else, the probe's own AEAD missing included, did not
// reach the question.
func classifyESPProbe(err error) kernelcap.Result {
	if errors.Is(err, unix.ESRCH) {
		return kernelcap.Result{State: kernelcap.StatePresent}
	}
	if errors.Is(err, unix.EPROTONOSUPPORT) {
		return kernelcap.Result{State: kernelcap.StateAbsent, Reason: err}
	}
	if err == nil {
		return kernelcap.Result{State: kernelcap.StateUnknown, Reason: errProbeUpdateAccepted}
	}
	return kernelcap.Result{State: kernelcap.StateUnknown, Reason: err}
}

// classifyTransformProbe reads the update's errno for one transform. ENOSYS is
// xfrm's algorithm lookup finding nothing; ENOENT is the crypto API failing to
// allocate the transform the ESP type asked for. Both are absence. EPERM, a
// missing ESP type and every other errno did not reach the question.
func classifyTransformProbe(err error) kernelcap.Result {
	if errors.Is(err, unix.ESRCH) {
		return kernelcap.Result{State: kernelcap.StatePresent}
	}
	if errors.Is(err, unix.ENOSYS) {
		return kernelcap.Result{State: kernelcap.StateAbsent, Reason: err}
	}
	if errors.Is(err, unix.ENOENT) {
		return kernelcap.Result{State: kernelcap.StateAbsent, Reason: err}
	}
	if err == nil {
		return kernelcap.Result{State: kernelcap.StateUnknown, Reason: errProbeUpdateAccepted}
	}
	return kernelcap.Result{State: kernelcap.StateUnknown, Reason: err}
}
