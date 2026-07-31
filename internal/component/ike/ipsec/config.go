// Design: plan/learned/734-ipsec-3-data-model.md -- IPsec config parser
// Related: algorithm_support.go -- which algorithms this build implements

package ipsec

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/secret"
)

var (
	errIPsecESPNoProposals  = errors.New("ipsec: esp-group has no proposals")
	errIPsecIKENoProposals  = errors.New("ipsec: ike-group has no proposals")
	errIPsecAuthModeMissing = errors.New("ipsec: authentication mode is required")
	errIPsecNameInvalid     = errors.New("ipsec: name contains invalid characters (allowed: alphanumeric, dash, underscore, dot)")
	errIPsecNameTooLong     = errors.New("ipsec: name exceeds 255 characters")
)

const (
	defaultESPLifetime uint32 = 3600
	defaultIKELifetime uint32 = 28800
	defaultDPDInterval uint16 = 30
	defaultDPDTimeout  uint16 = 120

	maxLifetime    uint32 = 86400
	maxDPDValue    uint16 = 3600
	maxProposalNum uint16 = 65535
	maxNameLen            = 255

	// MaxProposalsPerGroup bounds how many proposals one group can offer. The
	// Proposal Num field of RFC 7296 Section 3.3.1 is one octet. An offer numbers
	// its proposals one upward, so 255 is the last number a conforming offer can
	// carry. A larger group has no exact encoding. Ze refuses it here, rather than
	// wrap the number on the wire (ai/rules/exact-or-reject.md).
	MaxProposalsPerGroup = 255
)

func validateName(name string) error {
	if name == "" {
		return errIPsecNameInvalid
	}
	if len(name) > maxNameLen {
		return errIPsecNameTooLong
	}
	for _, c := range name {
		if !isNameChar(c) {
			return errIPsecNameInvalid
		}
	}
	return nil
}

func isNameChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.'
}

func emptyConfig() *IPsecConfig {
	return &IPsecConfig{
		ESPGroups: make(map[string]ESPGroup),
		IKEGroups: make(map[string]IKEGroup),
		Peers:     make(map[string]SiteToSitePeer),
	}
}

// ParseIPsecConfig extracts IPsec configuration from the parsed config tree.
// Returns an empty config if no vpn { ipsec {} } block is present.
func ParseIPsecConfig(tree *config.Tree) (*IPsecConfig, error) {
	if tree == nil {
		return emptyConfig(), nil
	}

	vpnRoot := tree.GetContainer("vpn")
	if vpnRoot == nil {
		return emptyConfig(), nil
	}

	ipsecRoot := vpnRoot.GetContainer("ipsec")
	if ipsecRoot == nil {
		return emptyConfig(), nil
	}

	cfg := &IPsecConfig{
		ESPGroups: make(map[string]ESPGroup),
		IKEGroups: make(map[string]IKEGroup),
		Peers:     make(map[string]SiteToSitePeer),
	}

	if v, ok := ipsecRoot.Get("interface"); ok {
		cfg.Interface = v
	}

	// RFC 7296 Section 2.6: how many half-open IKE SAs the responder tolerates before
	// it challenges an inbound initiation with a COOKIE. Absent means the YANG default
	// of zero, which challenges every initiation.
	if v, ok := ipsecRoot.Get("cookie-threshold"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("ipsec cookie-threshold %q: %w", v, err)
		}
		cfg.CookieThreshold = uint32(n)
	}

	for _, entry := range ipsecRoot.GetListOrdered("esp-group") {
		g, err := parseESPGroup(entry.Key, entry.Value)
		if err != nil {
			return nil, err
		}
		cfg.ESPGroups[entry.Key] = g
	}

	for _, entry := range ipsecRoot.GetListOrdered("ike-group") {
		g, err := parseIKEGroup(entry.Key, entry.Value)
		if err != nil {
			return nil, err
		}
		cfg.IKEGroups[entry.Key] = g
	}

	if stsRoot := ipsecRoot.GetContainer("site-to-site"); stsRoot != nil {
		for _, entry := range stsRoot.GetListOrdered("peer") {
			p, err := parseSiteToSitePeer(entry.Key, entry.Value)
			if err != nil {
				return nil, err
			}
			cfg.Peers[entry.Key] = p
		}
	}

	if raRoot := ipsecRoot.GetContainer("remote-access"); raRoot != nil {
		ra, err := parseRemoteAccess(raRoot)
		if err != nil {
			return nil, err
		}
		cfg.RemoteAccess = &ra
	}

	return cfg, nil
}

func parseESPGroup(name string, t *config.Tree) (ESPGroup, error) {
	if err := validateName(name); err != nil {
		return ESPGroup{}, fmt.Errorf("ipsec esp-group %q: %w", name, err)
	}

	g := ESPGroup{
		Name:     name,
		Lifetime: defaultESPLifetime,
		PFS:      PFSEnable,
	}

	if v, ok := t.Get("lifetime"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return g, fmt.Errorf("ipsec esp-group %q lifetime: %w", name, err)
		}
		if n > uint64(maxLifetime) {
			return g, fmt.Errorf("ipsec esp-group %q lifetime: %d exceeds maximum %d", name, n, maxLifetime)
		}
		g.Lifetime = uint32(n)
	}

	if v, ok := t.Get("pfs"); ok {
		pfs, valid := ParsePFSMode(v)
		if !valid {
			return g, fmt.Errorf("ipsec esp-group %q pfs: unsupported value %q", name, v)
		}
		g.PFS = pfs
	}

	proposals := t.GetListOrdered("proposal")
	if len(proposals) == 0 {
		return g, fmt.Errorf("ipsec esp-group %q: %w", name, errIPsecESPNoProposals)
	}
	if len(proposals) > MaxProposalsPerGroup {
		return g, fmt.Errorf("ipsec esp-group %q: %d proposals exceeds the limit of %d (RFC 7296 Section 3.3.1 gives Proposal Num one octet)",
			name, len(proposals), MaxProposalsPerGroup)
	}

	for _, entry := range proposals {
		p, err := parseESPProposal(name, entry.Key, entry.Value)
		if err != nil {
			return g, err
		}
		g.Proposals = append(g.Proposals, p)
	}

	sort.Slice(g.Proposals, func(i, j int) bool {
		return g.Proposals[i].Number < g.Proposals[j].Number
	})

	return g, nil
}

func parseESPProposal(groupName, numStr string, t *config.Tree) (ESPProposal, error) {
	num, err := strconv.ParseUint(numStr, 10, 16)
	if err != nil || num == 0 || num > uint64(maxProposalNum) {
		return ESPProposal{}, fmt.Errorf("ipsec esp-group %q proposal %q: invalid number (1-%d)",
			groupName, numStr, maxProposalNum)
	}

	p := ESPProposal{Number: uint16(num)}

	encStr, ok := t.Get("encryption")
	if !ok {
		return p, fmt.Errorf("ipsec esp-group %q proposal %d: encryption is required", groupName, num)
	}
	enc, valid := ParseEncryptionAlgo(encStr)
	if !valid {
		return p, fmt.Errorf("ipsec esp-group %q proposal %d: unsupported encryption algorithm %q",
			groupName, num, encStr)
	}
	if !EncryptionImplemented(enc) {
		return p, fmt.Errorf("ipsec esp-group %q proposal %d: encryption algorithm %q is not implemented by this build (implemented: %s)",
			groupName, num, encStr, strings.Join(SupportedEncryptionNames(), ", "))
	}
	p.Encryption = enc

	hashStr, hasHash := t.Get("hash")
	switch {
	case hasHash && enc.IsAEAD():
		// RFC 7296 Section 3.3: an AEAD cipher carries its own integrity, so the ESP
		// proposal offers the integrity transform NONE and a hash names nothing. ESP has
		// no PRF transform either, so the hash is not read as one. Ze once accepted the
		// spelling and derived integrity keys from it. That moved the responder
		// encryption key past the offset the peer reads (ai/rules/exact-or-reject.md).
		return p, fmt.Errorf("ipsec esp-group %q proposal %d: hash %q is not allowed beside the AEAD encryption algorithm %q; remove the hash",
			groupName, num, hashStr, encStr)
	case hasHash:
		h, valid := ParseHashAlgo(hashStr)
		if !valid {
			return p, fmt.Errorf("ipsec esp-group %q proposal %d: unsupported hash algorithm %q",
				groupName, num, hashStr)
		}
		if !HashImplemented(h) {
			return p, fmt.Errorf("ipsec esp-group %q proposal %d: hash algorithm %q is not implemented by this build (implemented: %s)",
				groupName, num, hashStr, strings.Join(SupportedHashNames(), ", "))
		}
		p.Hash = h
	case !enc.IsAEAD():
		// RFC 7296 Section 3.3: non-AEAD ciphers require a separate integrity algorithm.
		return p, fmt.Errorf("ipsec esp-group %q proposal %d: hash is required for non-AEAD encryption %q",
			groupName, num, encStr)
	}

	return p, nil
}

func parseIKEGroup(name string, t *config.Tree) (IKEGroup, error) {
	if err := validateName(name); err != nil {
		return IKEGroup{}, fmt.Errorf("ipsec ike-group %q: %w", name, err)
	}

	g := IKEGroup{
		Name:        name,
		KeyExchange: KeyExchangeIKEv2,
		Lifetime:    defaultIKELifetime,
		CloseAction: CloseActionNone,
		DPD: DPDConfig{
			Action:   DPDActionHold,
			Interval: defaultDPDInterval,
			Timeout:  defaultDPDTimeout,
		},
	}

	if v, ok := t.Get("key-exchange"); ok {
		ke, valid := ParseKeyExchange(v)
		if !valid {
			return g, fmt.Errorf("ipsec ike-group %q key-exchange: unsupported value %q", name, v)
		}
		g.KeyExchange = ke
	}

	if v, ok := t.Get("lifetime"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return g, fmt.Errorf("ipsec ike-group %q lifetime: %w", name, err)
		}
		if n > uint64(maxLifetime) {
			return g, fmt.Errorf("ipsec ike-group %q lifetime: %d exceeds maximum %d", name, n, maxLifetime)
		}
		g.Lifetime = uint32(n)
	}

	if v, ok := t.Get("close-action"); ok {
		ca, valid := ParseCloseAction(v)
		if !valid {
			return g, fmt.Errorf("ipsec ike-group %q close-action: unsupported value %q", name, v)
		}
		g.CloseAction = ca
	}

	if dpdTree := t.GetContainer("dead-peer-detection"); dpdTree != nil {
		dpd, err := parseDPD(name, dpdTree)
		if err != nil {
			return g, err
		}
		g.DPD = dpd
	}

	proposals := t.GetListOrdered("proposal")
	if len(proposals) == 0 {
		return g, fmt.Errorf("ipsec ike-group %q: %w", name, errIPsecIKENoProposals)
	}
	if len(proposals) > MaxProposalsPerGroup {
		return g, fmt.Errorf("ipsec ike-group %q: %d proposals exceeds the limit of %d (RFC 7296 Section 3.3.1 gives Proposal Num one octet)",
			name, len(proposals), MaxProposalsPerGroup)
	}

	for _, entry := range proposals {
		p, err := parseIKEProposal(name, entry.Key, entry.Value)
		if err != nil {
			return g, err
		}
		g.Proposals = append(g.Proposals, p)
	}

	sort.Slice(g.Proposals, func(i, j int) bool {
		return g.Proposals[i].Number < g.Proposals[j].Number
	})

	return g, nil
}

func parseDPD(groupName string, t *config.Tree) (DPDConfig, error) {
	dpd := DPDConfig{
		Action:   DPDActionHold,
		Interval: defaultDPDInterval,
		Timeout:  defaultDPDTimeout,
	}

	if v, ok := t.Get("action"); ok {
		a, valid := ParseDPDAction(v)
		if !valid {
			return dpd, fmt.Errorf("ipsec ike-group %q dead-peer-detection action: unsupported value %q",
				groupName, v)
		}
		dpd.Action = a
	}

	if v, ok := t.Get("interval"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return dpd, fmt.Errorf("ipsec ike-group %q dead-peer-detection interval: %w", groupName, err)
		}
		if n == 0 || n > uint64(maxDPDValue) {
			return dpd, fmt.Errorf("ipsec ike-group %q dead-peer-detection interval: %d out of range 1-%d",
				groupName, n, maxDPDValue)
		}
		dpd.Interval = uint16(n)
	}

	if v, ok := t.Get("timeout"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return dpd, fmt.Errorf("ipsec ike-group %q dead-peer-detection timeout: %w", groupName, err)
		}
		if n == 0 || n > uint64(maxDPDValue) {
			return dpd, fmt.Errorf("ipsec ike-group %q dead-peer-detection timeout: %d out of range 1-%d",
				groupName, n, maxDPDValue)
		}
		dpd.Timeout = uint16(n)
	}

	return dpd, nil
}

func parseIKEProposal(groupName, numStr string, t *config.Tree) (IKEProposal, error) {
	num, err := strconv.ParseUint(numStr, 10, 16)
	if err != nil || num == 0 || num > uint64(maxProposalNum) {
		return IKEProposal{}, fmt.Errorf("ipsec ike-group %q proposal %q: invalid number (1-%d)",
			groupName, numStr, maxProposalNum)
	}

	p := IKEProposal{Number: uint16(num)}

	encStr, ok := t.Get("encryption")
	if !ok {
		return p, fmt.Errorf("ipsec ike-group %q proposal %d: encryption is required", groupName, num)
	}
	enc, valid := ParseEncryptionAlgo(encStr)
	if !valid {
		return p, fmt.Errorf("ipsec ike-group %q proposal %d: unsupported encryption algorithm %q",
			groupName, num, encStr)
	}
	if !EncryptionImplemented(enc) {
		return p, fmt.Errorf("ipsec ike-group %q proposal %d: encryption algorithm %q is not implemented by this build (implemented: %s)",
			groupName, num, encStr, strings.Join(SupportedEncryptionNames(), ", "))
	}
	p.Encryption = enc

	// An IKE proposal reads its hash as the PRF, which RFC 7296 Section 3.3.3 makes a
	// mandatory transform for every cipher. The hash is therefore required beside an
	// AEAD cipher here, where an ESP proposal refuses it.
	hashStr, ok := t.Get("hash")
	if !ok {
		return p, fmt.Errorf("ipsec ike-group %q proposal %d: hash is required", groupName, num)
	}
	h, valid := ParseHashAlgo(hashStr)
	if !valid {
		return p, fmt.Errorf("ipsec ike-group %q proposal %d: unsupported hash algorithm %q",
			groupName, num, hashStr)
	}
	if !HashImplemented(h) {
		return p, fmt.Errorf("ipsec ike-group %q proposal %d: hash algorithm %q is not implemented by this build (implemented: %s)",
			groupName, num, hashStr, strings.Join(SupportedHashNames(), ", "))
	}
	p.Hash = h

	dhStr, ok := t.Get("dh-group")
	if !ok {
		return p, fmt.Errorf("ipsec ike-group %q proposal %d: dh-group is required", groupName, num)
	}
	dh, err := strconv.ParseUint(dhStr, 10, 8)
	if err != nil {
		return p, fmt.Errorf("ipsec ike-group %q proposal %d dh-group: %w", groupName, num, err)
	}
	g := DHGroup(dh)
	if !ValidDHGroup(g) {
		return p, fmt.Errorf("ipsec ike-group %q proposal %d: unsupported DH group %d (valid: 1-31)",
			groupName, num, dh)
	}
	p.DHGroup = g

	return p, nil
}

func parseSiteToSitePeer(name string, t *config.Tree) (SiteToSitePeer, error) {
	if err := validateName(name); err != nil {
		return SiteToSitePeer{}, fmt.Errorf("ipsec peer %q: %w", name, err)
	}

	peer := SiteToSitePeer{
		Name:           name,
		ConnectionType: ConnectionInitiate,
	}

	if v, ok := t.Get("ike-group"); ok {
		peer.IKEGroup = v
	}
	if v, ok := t.Get("esp-group"); ok {
		peer.ESPGroup = v
	}

	if v, ok := t.Get("connection-type"); ok {
		ct, valid := ParseConnectionType(v)
		if !valid {
			return peer, fmt.Errorf("ipsec peer %q connection-type: unsupported value %q", name, v)
		}
		peer.ConnectionType = ct
	}

	if v, ok := t.Get("local-address"); ok {
		peer.LocalAddress = v
	}
	if v, ok := t.Get("remote-address"); ok {
		peer.RemoteAddress = v
	}

	if authTree := t.GetContainer("authentication"); authTree != nil {
		auth, err := parseAuthConfig(name, authTree)
		if err != nil {
			return peer, err
		}
		peer.Auth = auth
	}

	if vtiTree := t.GetContainer("vti"); vtiTree != nil {
		if v, ok := vtiTree.Get("bind"); ok {
			peer.VTIBind = v
		}
	}

	return peer, nil
}

func parseRemoteAccess(t *config.Tree) (RemoteAccessConfig, error) {
	ra := RemoteAccessConfig{
		Users: make(map[string]EAPUser),
	}

	if v, ok := t.Get("ike-group"); ok {
		ra.IKEGroup = v
	}
	if v, ok := t.Get("esp-group"); ok {
		ra.ESPGroup = v
	}

	if authTree := t.GetContainer("authentication"); authTree != nil {
		auth, err := parseAuthConfig("remote-access", authTree)
		if err != nil {
			return ra, err
		}
		ra.Auth = auth
	}

	if pools := t.GetListOrdered("pool"); len(pools) > 0 {
		pool, err := parseVirtualIPPool(pools[0].Key, pools[0].Value)
		if err != nil {
			return ra, err
		}
		ra.Pool = pool
	}

	for _, entry := range t.GetListOrdered("eap-user") {
		user, err := parseEAPUser(entry.Key, entry.Value, ra.Auth.Mode)
		if err != nil {
			return ra, err
		}
		ra.Users[entry.Key] = user
	}

	return ra, nil
}

func parseVirtualIPPool(name string, t *config.Tree) (VirtualIPPool, error) {
	if err := validateName(name); err != nil {
		return VirtualIPPool{}, fmt.Errorf("ipsec pool %q: %w", name, err)
	}

	pool := VirtualIPPool{Name: name}

	if v, ok := t.Get("range"); ok {
		pool.Range = v
	}
	if v, ok := t.Get("range6"); ok {
		pool.Range6 = v
	}

	if v, ok := t.Get("dns"); ok {
		pool.DNS = append(pool.DNS, v)
	}

	if v, ok := t.Get("domain"); ok {
		pool.Domain = v
	}

	return pool, nil
}

func parseEAPUser(name string, t *config.Tree, authMode AuthMode) (EAPUser, error) {
	if err := validateName(name); err != nil {
		return EAPUser{}, fmt.Errorf("ipsec eap-user %q: %w", name, err)
	}

	user := EAPUser{Name: name}

	switch authMode {
	case AuthEAPMSCHAPv2:
		if v, ok := t.Get("password"); ok {
			if secret.IsEncoded(v) {
				decoded, err := secret.Decode(v)
				if err != nil {
					return user, fmt.Errorf("ipsec eap-user %q password decode: %w", name, err)
				}
				user.Password = decoded
			} else {
				user.Password = v
			}
		}
	case AuthEAPTLS:
		if v, ok := t.Get("certificate"); ok {
			user.Certificate = v
		}
	case AuthUnknown, AuthPreSharedSecret, AuthX509:
	}

	return user, nil
}

func parseAuthConfig(peerName string, t *config.Tree) (AuthConfig, error) {
	var auth AuthConfig

	modeStr, ok := t.Get("mode")
	if !ok {
		return auth, fmt.Errorf("ipsec peer %q: %w", peerName, errIPsecAuthModeMissing)
	}
	mode, valid := ParseAuthMode(modeStr)
	if !valid {
		return auth, fmt.Errorf("ipsec peer %q authentication mode: unsupported value %q", peerName, modeStr)
	}
	auth.Mode = mode

	if v, ok := t.Get("local-id"); ok {
		auth.LocalID = v
	}
	if v, ok := t.Get("remote-id"); ok {
		auth.RemoteID = v
	}

	switch mode {
	case AuthPreSharedSecret:
		if v, ok := t.Get("pre-shared-secret"); ok {
			if secret.IsEncoded(v) {
				decoded, err := secret.Decode(v)
				if err != nil {
					return auth, fmt.Errorf("ipsec peer %q pre-shared-secret decode: %w", peerName, err)
				}
				auth.PSK = decoded
			} else {
				auth.PSK = v
			}
		}
	case AuthEAPMSCHAPv2:
		if v, ok := t.Get("pre-shared-secret"); ok {
			if secret.IsEncoded(v) {
				decoded, err := secret.Decode(v)
				if err != nil {
					return auth, fmt.Errorf("ipsec peer %q pre-shared-secret decode: %w", peerName, err)
				}
				auth.PSK = decoded
			} else {
				auth.PSK = v
			}
		}
		if v, ok := t.Get("ca-certificate"); ok {
			auth.CACertificate = v
		}
		if v, ok := t.Get("certificate"); ok {
			auth.Certificate = v
		}
		if x509Tree := t.GetContainer("x509"); x509Tree != nil {
			if auth.CACertificate == "" {
				if v, ok := x509Tree.Get("ca-certificate"); ok {
					auth.CACertificate = v
				}
			}
			if auth.Certificate == "" {
				if v, ok := x509Tree.Get("certificate"); ok {
					auth.Certificate = v
				}
			}
		}
	case AuthX509, AuthEAPTLS:
		if v, ok := t.Get("ca-certificate"); ok {
			auth.CACertificate = v
		}
		if v, ok := t.Get("certificate"); ok {
			auth.Certificate = v
		}
		if x509Tree := t.GetContainer("x509"); x509Tree != nil {
			if auth.CACertificate == "" {
				if v, ok := x509Tree.Get("ca-certificate"); ok {
					auth.CACertificate = v
				}
			}
			if auth.Certificate == "" {
				if v, ok := x509Tree.Get("certificate"); ok {
					auth.Certificate = v
				}
			}
		}
	case AuthUnknown:
	}

	return auth, nil
}
