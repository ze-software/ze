// Design: docs/architecture/testing/interop.md — scenario rendering and placeholders
// Related: ../../le/interoplab/bgp/prepare.go — the BGP lab renders link-locals through this package
// Related: ../../le/interoplab/ipsec/ipsec.go — the IPsec lab renders PKI material through this package
// Related: ../../component/config/cli/validate_corpus_test.go — the config sweep renders the same placeholders

// Package labconfig holds the placeholder grammar interop scenario files use,
// and the renderer that fills it in.
//
// A scenario file cannot name an address or a certificate it only learns at
// run time, so it writes a placeholder the lab replaces before a container
// starts. The grammar and the renderer live here, outside internal/le, so
// both the lab (internal/le/interoplab) and a test that validates the
// scenario's Ze config (the repository config sweep) render the same text the
// same way, from one declaration.
package labconfig

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// LinkLocal is one link-local placeholder and the host number its owner's
// address carries in its interface identifier.
type LinkLocal struct {
	Placeholder string
	Host        uint8
}

// The owners of a link-local placeholder, as indexes into LinkLocals, so a
// table indexed by owner is written by name, never by position.
const (
	ZeLinkLocal = iota
	FRRLinkLocal
	FRRSinkLinkLocal
	linkLocalCount
)

// The host number of each container a placeholder names: the last byte of
// its lab IPv4 address and of its link-local interface identifier.
const (
	ZeHost      uint8 = 2
	FRRHost     uint8 = 3
	FRRSinkHost uint8 = 15
)

// The link-local placeholders a scenario file may carry.
const (
	ZeLinkLocalPlaceholder      = "@ZE_LINK_LOCAL@"
	FRRLinkLocalPlaceholder     = "@FRR_LINK_LOCAL@"
	FRRSinkLinkLocalPlaceholder = "@FRR_SINK_LINK_LOCAL@"
)

// LinkLocals is every link-local placeholder with its owner. Each
// renders to the address LinkLocalAt gives its owner's host under the lab's
// prefix.
var LinkLocals = [linkLocalCount]LinkLocal{
	ZeLinkLocal:      {Placeholder: ZeLinkLocalPlaceholder, Host: ZeHost},
	FRRLinkLocal:     {Placeholder: FRRLinkLocalPlaceholder, Host: FRRHost},
	FRRSinkLinkLocal: {Placeholder: FRRSinkLinkLocalPlaceholder, Host: FRRSinkHost},
}

// RenderedLinkLocals records, by index into LinkLocals, which placeholders a
// rendered text carried.
type RenderedLinkLocals [linkLocalCount]bool

// LinkLocalAt puts the lab's IPv6 prefix and a container's host number in the
// interface identifier of an fe80::/64 address. The lab assigns it to that
// container's eth0 before startup; rendering and the checkers use this same
// address rather than an unowned literal.
func LinkLocalAt(prefix netip.Prefix, host uint8) netip.Addr {
	octets := prefix.Masked().Addr().As16()
	return netip.AddrFrom16([16]byte{
		0xfe, 0x80,
		8: octets[0], 9: octets[1], 10: octets[2], 11: octets[3],
		12: octets[4], 13: octets[5], 15: host,
	})
}

// RenderLinkLocals replaces every link-local placeholder in text with its
// owner's address under prefix, and answers which placeholders text carried.
// A placeholder with no valid IPv6 prefix to render it from is refused: the
// lab cannot give that container an address.
func RenderLinkLocals(text string, prefix netip.Prefix) (string, RenderedLinkLocals, error) {
	var rendered RenderedLinkLocals
	for owner := range LinkLocals {
		placeholder := LinkLocals[owner].Placeholder
		if !strings.Contains(text, placeholder) {
			continue
		}
		if !prefix.IsValid() {
			return "", rendered, errors.New(placeholder + " needs an IPv6 lab network, and the scenario selected none")
		}
		text = strings.ReplaceAll(text, placeholder, LinkLocalAt(prefix, LinkLocals[owner].Host).String())
		rendered[owner] = true
	}
	return text, rendered, nil
}

// pkiPlaceholder is the PKI placeholder: %%PKI_B64:<file>%% renders to the
// base64 DER body of the one PEM block <file> holds.
var pkiPlaceholder = regexp.MustCompile(`%%PKI_B64:([A-Za-z0-9._-]+)%%`)

// pemHeader matches an RFC 1421 header line inside a PEM block, which only an
// encrypted block carries.
var pemHeader = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*:`)

// FindPKIDir answers the directory a scenario's PKI placeholders read from:
// the scenario's own pki directory, else the shared test/interop-ipsec/pki
// under the repository root, and "" when neither exists.
func FindPKIDir(root, scenarioDir string) string {
	local := filepath.Join(scenarioDir, "pki")
	if directoryExists(local) {
		return local
	}
	shared := filepath.Join(root, "test", "interop-ipsec", "pki")
	if directoryExists(shared) && fileExists(filepath.Join(shared, "ca.pem")) {
		return shared
	}
	return ""
}

// RenderPKIPlaceholders replaces every PKI placeholder in content with the
// base64 DER body of the file it names under pkiDir. Content with no
// placeholder is answered unchanged; a placeholder with no pkiDir, or naming a
// file that is missing or holds anything but one unencrypted PEM block, is
// refused.
func RenderPKIPlaceholders(content, pkiDir string) (string, error) {
	matches := pkiPlaceholder.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content, nil
	}
	if pkiDir == "" {
		return "", errors.New("config needs PKI material and the scenario has no pki directory")
	}
	cache := make(map[string]string)
	for _, match := range matches {
		name := match[1]
		if _, ok := cache[name]; ok {
			continue
		}
		path := filepath.Join(pkiDir, name)
		text, err := readFileUnder(pkiDir, name)
		if err != nil {
			return "", fmt.Errorf("cannot read PKI file %s: %w", path, err)
		}
		body, err := PEMBase64DER(string(text), path)
		if err != nil {
			return "", err
		}
		cache[name] = body
	}
	return pkiPlaceholder.ReplaceAllStringFunc(content, func(token string) string {
		name := pkiPlaceholder.FindStringSubmatch(token)[1]
		return cache[name]
	}), nil
}

// PEMBase64DER answers the base64 DER body of the one PEM block text holds,
// skipping an EC PARAMETERS prelude. Encrypted, empty, bundled or corrupt
// material is refused naming source, so it never becomes a pki leaf.
func PEMBase64DER(text, source string) (string, error) {
	type block struct{ label, body string }
	var blocks []block
	label := ""
	var body textbuf.Buffer
	for raw := range strings.SplitSeq(text, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "-----BEGIN ") && strings.HasSuffix(line, "-----") {
			label = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "-----BEGIN "), "-----"))
			if label == "ENCRYPTED PRIVATE KEY" {
				return "", fmt.Errorf("%s: PEM block %q holds encrypted key material", source, label)
			}
			body.Reset()
			continue
		}
		if strings.HasPrefix(line, "-----END ") && strings.HasSuffix(line, "-----") {
			if label == "" {
				continue
			}
			if label != "EC PARAMETERS" {
				if body.Len() == 0 {
					return "", fmt.Errorf("%s: PEM block %q holds no data", source, label)
				}
				blocks = append(blocks, block{label: label, body: body.String()})
			}
			label = ""
			body.Reset()
			continue
		}
		if label == "" || line == "" {
			continue
		}
		if pemHeader.MatchString(line) {
			return "", fmt.Errorf("%s: PEM block %q carries encrypted RFC 1421 header %q", source, label, line)
		}
		body.Str(line)
	}
	if len(blocks) == 0 {
		return "", fmt.Errorf("%s: no complete PEM block found", source)
	}
	if len(blocks) != 1 {
		return "", fmt.Errorf("%s: the file holds %d PEM blocks, and a pki leaf holds one value", source, len(blocks))
	}
	if _, err := base64.StdEncoding.DecodeString(blocks[0].body); err != nil {
		return "", fmt.Errorf("%s: PEM block %q does not hold valid base64: %w", source, blocks[0].label, err)
	}
	return blocks[0].body, nil
}

// readFileUnder reads name inside directory without following a path out of it.
func readFileUnder(directory, name string) ([]byte, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	content, readErr := root.ReadFile(name)
	return content, errors.Join(readErr, root.Close())
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
