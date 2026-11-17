// Design: docs/architecture/container/image-store.md -- the reference forms and their completion
// Related: client.go -- the client that fetches what a reference names

// Package registry is the pull-only client of a container registry: it
// completes an image reference, resolves it to the manifest of the running
// platform, and fetches the manifest, the image configuration and the layer
// blobs, each checked against its digest and its size as the bytes arrive.
// It speaks the distribution protocol over the standard library's HTTP
// client, with its own redirect and credential rules, and stores nothing:
// the image store decides where a blob goes.
package registry

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// The default registry, the public hub, as an operator writes it and as the
// client reaches it. A name of one component is placed under its library
// namespace, as the hub itself does.
const (
	hubName      = "docker.io"
	hubAlias     = "index.docker.io"
	hubEndpoint  = "registry-1.docker.io"
	hubLibrary   = "library/"
	defaultTag   = "latest"
	localhostKey = "localhost"
)

// Bounds of a reference. The repository bound is the distribution
// protocol's name bound, the tag bound its tag grammar's.
const (
	referenceLengthMax  = 1024
	repositoryLengthMax = 255
	tagLengthMax        = 128
)

// The digest form every object is named by. Only SHA-256 is accepted.
const (
	digestAlgorithm = "sha256"
	digestHexLength = 64
	// digestAlgorithmShownMax bounds the part of a refused algorithm that an
	// error repeats, since the text came from outside.
	digestAlgorithmShownMax = 32
)

// Digest is a SHA-256 digest that was checked when it was built: only
// ParseDigest and digestOf build one, so a value of this type is always
// sha256 followed by 64 lower-case hex digits. The zero Digest is no digest.
type Digest struct {
	hex string
}

// ParseDigest reads sha256:<64 lower-case hex digits>. Another algorithm is
// refused by name; upper case is refused rather than folded, because two
// spellings of one digest would name two folders of the store.
func ParseDigest(text string) (Digest, error) {
	algorithm, encoded, found := strings.Cut(text, ":")
	if !found {
		return Digest{}, fmt.Errorf("digest %q is not algorithm:hex", shown(text))
	}
	if algorithm != digestAlgorithm {
		return Digest{}, fmt.Errorf("digest algorithm %q is not accepted: only sha256 is", shown(algorithm))
	}
	if len(encoded) != digestHexLength {
		return Digest{}, fmt.Errorf("digest %q: want %d hex digits, got %d", shown(text), digestHexLength, len(encoded))
	}
	if strings.ToLower(encoded) != encoded {
		return Digest{}, fmt.Errorf("digest %q is not in lower case", shown(text))
	}
	if _, err := hex.DecodeString(encoded); err != nil {
		return Digest{}, fmt.Errorf("digest %q holds a character that is not a hex digit", shown(text))
	}
	return Digest{hex: encoded}, nil
}

// String answers sha256:<hex>, or the empty string for the zero Digest.
func (d Digest) String() string {
	if d.hex == "" {
		return ""
	}
	return digestAlgorithm + ":" + d.hex
}

// IsZero answers whether d is no digest.
func (d Digest) IsZero() bool {
	return d.hex == ""
}

// Reference is a completed image reference: a registry host, a repository,
// and a tag, a digest or both. Only ParseReference builds one, so the short
// and the whole spelling of one image are one value.
type Reference struct {
	registry   string
	repository string
	tag        string
	digest     Digest
}

// ParseReference completes text by the reference table of the image store's
// design: the first component is a registry host when it holds a dot or a
// colon or is localhost, otherwise the name belongs to the public hub, where
// a name of one component goes under the library namespace. A name with no
// tag and no digest takes the tag latest. Nothing is corrected: a part that
// does not match its grammar refuses the reference, naming the part.
func ParseReference(text string) (Reference, error) {
	if text == "" {
		return Reference{}, errors.New("image reference is empty")
	}
	if len(text) > referenceLengthMax {
		return Reference{}, fmt.Errorf("image reference is %d bytes, more than %d", len(text), referenceLengthMax)
	}
	name, digestText, pinned := strings.Cut(text, "@")
	var ref Reference
	if pinned {
		digest, err := ParseDigest(digestText)
		if err != nil {
			return Reference{}, fmt.Errorf("image reference %q: %w", text, err)
		}
		ref.digest = digest
	}
	if colon := strings.LastIndex(name, ":"); colon > strings.LastIndex(name, "/") {
		ref.tag = name[colon+1:]
		name = name[:colon]
		if err := checkTag(ref.tag); err != nil {
			return Reference{}, fmt.Errorf("image reference %q: %w", text, err)
		}
	}
	ref.registry, ref.repository = splitRegistry(name)
	if err := checkHost(ref.registry); err != nil {
		return Reference{}, fmt.Errorf("image reference %q: registry: %w", text, err)
	}
	if err := checkRepository(ref.repository); err != nil {
		return Reference{}, fmt.Errorf("image reference %q: repository: %w", text, err)
	}
	if ref.tag == "" && ref.digest.IsZero() {
		ref.tag = defaultTag
	}
	return ref, nil
}

// splitRegistry applies the host rule and completes a name of the hub.
func splitRegistry(name string) (registry, repository string) {
	first, rest, found := strings.Cut(name, "/")
	isHost := found && (strings.ContainsAny(first, ".:") || first == localhostKey)
	if !isHost {
		first, rest = hubName, name
	}
	if first == hubAlias {
		first = hubName
	}
	if first == hubName && !strings.Contains(rest, "/") {
		rest = hubLibrary + rest
	}
	return first, rest
}

// Registry answers the registry host, as the reference completes it.
func (r Reference) Registry() string { return r.registry }

// Repository answers the repository path inside the registry.
func (r Reference) Repository() string { return r.repository }

// Tag answers the tag, written or implied, and the empty string for a
// reference pinned by digest alone.
func (r Reference) Tag() string { return r.tag }

// Digest answers the digest the reference pins, and false when it pins none.
func (r Reference) Digest() (Digest, bool) { return r.digest, !r.digest.IsZero() }

// pin answers what the manifest request names: the digest when there is
// one, because it is never resolved again, and the tag otherwise.
func (r Reference) pin() string {
	if !r.digest.IsZero() {
		return r.digest.String()
	}
	return r.tag
}

// String answers the whole form: registry/repository[:tag][@digest].
func (r Reference) String() string {
	text := r.registry + "/" + r.repository
	if r.tag != "" {
		text += ":" + r.tag
	}
	if !r.digest.IsZero() {
		text += "@" + r.digest.String()
	}
	return text
}

// endpointHost answers the host the client dials for a registry name: the
// hub is written docker.io and served from another host.
func endpointHost(registry string) string {
	if registry == hubName {
		return hubEndpoint
	}
	return registry
}

// checkTag refuses a tag outside [A-Za-z0-9_][A-Za-z0-9_.-]{0,127}.
func checkTag(tag string) error {
	if tag == "" {
		return errors.New("tag is empty")
	}
	if len(tag) > tagLengthMax {
		return fmt.Errorf("tag is %d characters, more than %d", len(tag), tagLengthMax)
	}
	for i, c := range tag {
		if !isTagCharacter(c, i == 0) {
			return fmt.Errorf("tag %q holds %q, which a tag cannot hold there", tag, c)
		}
	}
	return nil
}

// isTagCharacter answers whether c may stand in a tag: a letter of either
// case, a digit or an underscore anywhere, a dot or a dash after the first.
func isTagCharacter(c rune, first bool) bool {
	switch {
	case isAlnum(c), c >= 'A' && c <= 'Z', c == '_':
		return true
	case c == '.', c == '-':
		return !first
	}
	return false
}

// checkRepository refuses a repository whose components are not lower-case
// letters and digits joined by one dot, one or two underscores, or dashes.
func checkRepository(repository string) error {
	if len(repository) > repositoryLengthMax {
		return fmt.Errorf("%d characters, more than %d", len(repository), repositoryLengthMax)
	}
	for component := range strings.SplitSeq(repository, "/") {
		if err := checkComponent(component); err != nil {
			return fmt.Errorf("%q: %w", repository, err)
		}
	}
	return nil
}

// checkComponent checks one component of a repository path.
func checkComponent(component string) error {
	if component == "" {
		return errors.New("a component is empty")
	}
	separator := ""
	for i, c := range component {
		if c >= 'A' && c <= 'Z' {
			return fmt.Errorf("component %q holds an upper-case letter: a repository is lower case", component)
		}
		if isAlnum(c) {
			if err := checkSeparator(component, separator, i); err != nil {
				return err
			}
			separator = ""
			continue
		}
		if c != '.' && c != '_' && c != '-' {
			return fmt.Errorf("component %q holds %q", component, c)
		}
		separator += string(c)
	}
	if separator != "" {
		return fmt.Errorf("component %q ends with a separator", component)
	}
	return nil
}

// checkSeparator judges the separator run that ends before index i of
// component: none, a dot, one or two underscores, or any run of dashes, and
// never at the start.
func checkSeparator(component, separator string, i int) error {
	if separator == "" {
		return nil
	}
	if i == len(separator) {
		return fmt.Errorf("component %q starts with a separator", component)
	}
	switch separator {
	case ".", "_", "__":
		return nil
	}
	if strings.Trim(separator, "-") == "" {
		return nil
	}
	return fmt.Errorf("component %q holds the separator %q", component, separator)
}

// checkHost refuses a registry that is not host[:port], where host is a DNS
// name in lower case, localhost, or a bracketed IPv6 address.
func checkHost(host string) error {
	name, port, hasPort := host, "", false
	if rest, bracketed := strings.CutPrefix(host, "["); bracketed {
		address, after, closed := strings.Cut(rest, "]")
		if !closed {
			return fmt.Errorf("%q opens an IPv6 address it does not close", host)
		}
		parsed, err := netip.ParseAddr(address)
		if err != nil {
			return fmt.Errorf("%q is not a bracketed IPv6 address: %w", host, err)
		}
		if !parsed.Is6() {
			return fmt.Errorf("%q brackets an address that is not IPv6", host)
		}
		if after != "" {
			port, hasPort = strings.CutPrefix(after, ":")
			if !hasPort {
				return fmt.Errorf("%q holds text after the IPv6 address", host)
			}
		}
		return checkPort(host, port, hasPort)
	}
	if before, after, found := strings.CutLast(host, ":"); found {
		name, port, hasPort = before, after, true
	}
	if err := checkHostName(name); err != nil {
		return fmt.Errorf("%q: %w", host, err)
	}
	return checkPort(host, port, hasPort)
}

// checkHostName checks the DNS labels of a registry host.
func checkHostName(name string) error {
	if name == "" {
		return errors.New("the host is empty")
	}
	for label := range strings.SplitSeq(name, ".") {
		if label == "" {
			return errors.New("a label of the host is empty")
		}
		if strings.HasPrefix(label, "-") {
			return fmt.Errorf("the label %q starts with a dash", label)
		}
		if strings.HasSuffix(label, "-") {
			return fmt.Errorf("the label %q ends with a dash", label)
		}
		for _, c := range label {
			if c >= 'A' && c <= 'Z' {
				return fmt.Errorf("the label %q holds an upper-case letter: write the host in lower case", label)
			}
			if !isAlnum(c) && c != '-' {
				return fmt.Errorf("the label %q holds %q", label, c)
			}
		}
	}
	return nil
}

// checkPort checks the port of a registry host, when it has one.
func checkPort(host, port string, hasPort bool) error {
	if !hasPort {
		return nil
	}
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return fmt.Errorf("%q: the port %q is not a number from 1 to 65535", host, port)
	}
	if number == 0 {
		return fmt.Errorf("%q: the port 0 cannot be reached", host)
	}
	return nil
}

// isAlnum answers whether c is a lower-case letter or a digit.
func isAlnum(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

// shown bounds text from outside before an error repeats it.
func shown(text string) string {
	if len(text) > digestAlgorithmShownMax+digestHexLength+1 {
		return text[:digestAlgorithmShownMax] + "..."
	}
	return text
}
