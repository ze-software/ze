// Design: docs/architecture/container/image-store.md -- layer application: the rules of the application table
// Related: apply_linux.go -- the applier that runs these rules on a staged root
// Related: publish.go -- the counts the applier takes go into the completion mark

// Linux only, as the applier is: it resolves every entry with
// descriptor-relative calls, and containers run on Linux alone. The fetch
// worker, which is the first caller outside the tests, decides what another
// system answers.

//go:build linux

package imagestore

import (
	"compress/gzip"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// The bounds of one image. Each refusal names the bound it passed.
const (
	// entriesDefault is the most entries one image applies, over all its
	// layers: the archive entries, and the folders on the way to them that a
	// layer names for the first time, because each of those may be a folder
	// the store creates.
	entriesDefault = 500_000
	// layersMax is the most layers one image has.
	layersMax = 128
	// entryNameMax is the longest name of one path component, in bytes.
	entryNameMax = 255
	// entryPathMax is the longest path of one entry, and the longest target of
	// a symbolic link, in bytes.
	entryPathMax = 4096
	// entryAttributesMax bounds the extended attributes of one entry, names
	// and values together, in bytes.
	entryAttributesMax = 64 << 10
	// zstdWindowMax is the largest window a zstd layer may ask the decoder
	// for, so a layer cannot make the decoder take more memory than this.
	zstdWindowMax = 32 << 20
	// ownerMax is the largest owner or group number. The next one, all bits
	// set, tells chown to leave the owner unchanged, so it names nobody.
	ownerMax = 1<<32 - 2
	// entryDisplayMax bounds the part of an entry's name an error shows: the
	// name is the image's, so it is quoted and cut, never shown raw.
	entryDisplayMax = 256
)

// The whiteout family of names, from the OCI image layer format.
const (
	// whiteoutPrefix starts the name of a whiteout and of every marker.
	whiteoutPrefix = ".wh."
	// opaqueMarker is the name, after whiteoutPrefix, of the marker that
	// makes its folder opaque.
	opaqueMarker = ".wh..opq"
	// devicesFolder is the image's device folder. The runtime mounts its own
	// there, so a node an image holds under it could never be seen.
	devicesFolder = "dev"
)

// The PAX record prefixes that carry an extended attribute. The first is the
// form every layer writer uses. The second is the libarchive form, whose
// names are encoded, which the store does not read: an attribute it cannot
// judge refuses the image.
const (
	paxAttributePrefix           = "SCHILY.xattr."
	paxLibarchiveAttributePrefix = "LIBARCHIVE.xattr."
)

// The attributes an image may carry. Every other name refuses the image.
const (
	attributeFileCapability = "security.capability"
	attributeACLAccess      = "system.posix_acl_access"
	attributeACLDefault     = "system.posix_acl_default"
	attributeUserPrefix     = "user."
	// The overlay keeps its own control attributes in these two namespaces.
	// An image that carries one could change what the overlay of a container
	// shows, so it refuses the image.
	attributeOverlayTrusted = "trusted.overlay."
	attributeOverlayUser    = "user.overlay."
)

// applyLimits are the bounds of one image that are not constants: the
// largest image size is the operator's, and the entry bound is a field so a
// test can reach it without creating half a million files.
type applyLimits struct {
	// expandedBytesMax bounds the decompressed bytes of every layer of the
	// image together, and the bytes written into its files.
	expandedBytesMax uint64
	// entriesMax bounds the entries of every layer together.
	entriesMax uint64
}

// defaultApplyLimits answers the bounds of an image whose largest size is
// largestImage.
func defaultApplyLimits(largestImage uint64) applyLimits {
	return applyLimits{expandedBytesMax: largestImage, entriesMax: entriesDefault}
}

// applyCounts are the totals of one image, over the layers applied so far.
type applyCounts struct {
	// expandedBytes is the count of decompressed bytes of the layers: what
	// the completion mark records as the image's expanded bytes.
	expandedBytes uint64
	// contentBytes is the count of bytes written into files. A sparse entry
	// writes more than the archive holds, so it has a count of its own.
	contentBytes uint64
	// entries is the count of archive entries.
	entries uint64
	// impliedFolders is the count of folders on the way to an entry that a
	// layer named for the first time. With entries it makes the entry bound.
	impliedFolders uint64
	// devicesSkipped is the count of device nodes under the device folder
	// that were not created.
	devicesSkipped uint64
	// layers is the count of layers begun.
	layers uint64
}

// refusalRule is the rule of the application table an image broke.
type refusalRule uint8

const (
	ruleUnspecified refusalRule = iota
	ruleAbsolutePath
	ruleParentComponent
	ruleEmptyComponent
	ruleNULInName
	ruleNameLength
	rulePathLength
	ruleEntryType
	ruleRootEntry
	ruleDeviceNode
	ruleHardLinkTarget
	ruleLinkTarget
	ruleWhiteoutTarget
	ruleOwner
	ruleAttributeName
	ruleOverlayAttribute
	ruleAttributeBytes
	ruleAttributeOnType
	ruleExpandedBytes
	ruleEntries
	ruleLayers
	ruleDiffDigest
	ruleCompression
	ruleArchive
)

// refusalSentences says each rule in the words an operator reads.
var refusalSentences = [...]string{
	ruleUnspecified:      "BUG: a refusal with no rule",
	ruleAbsolutePath:     "an absolute path is refused: an entry names a place inside the image",
	ruleParentComponent:  "a path with a parent-directory component is refused",
	ruleEmptyComponent:   "a path with an empty component is refused",
	ruleNULInName:        "a name holding a NUL byte is refused",
	ruleNameLength:       "a name longer than 255 bytes is refused",
	rulePathLength:       "a path longer than 4096 bytes is refused",
	ruleEntryType:        "an entry of this type is refused",
	ruleRootEntry:        "only a folder may name the root of the image",
	ruleDeviceNode:       "a device node outside the image's dev folder is refused",
	ruleHardLinkTarget:   "a hard link must name a regular file that exists inside the image",
	ruleLinkTarget:       "a symbolic link must have a target of 1 to 4096 bytes with no NUL byte",
	ruleWhiteoutTarget:   "a whiteout must name one entry of its folder",
	ruleOwner:            "an owner or a group must be a number from 0 to 4294967294",
	ruleAttributeName:    "an extended attribute of this name is refused",
	ruleOverlayAttribute: "an overlay control attribute is refused",
	ruleAttributeBytes:   "extended attributes over 64 KiB on one entry are refused",
	ruleAttributeOnType:  "an extended attribute is kept on a file or a folder only",
	ruleExpandedBytes:    "the image expands past the largest image size",
	ruleEntries:          "the image holds more entries than the bound",
	ruleLayers:           "an image of more than 128 layers is refused",
	ruleDiffDigest:       "a decompressed layer does not hash to the digest the image configuration declares",
	ruleCompression:      "the layer cannot be decompressed",
	ruleArchive:          "the layer is not a readable tar archive",
}

// String answers the rule's sentence.
func (r refusalRule) String() string {
	if int(r) < len(refusalSentences) {
		return refusalSentences[r]
	}
	return "BUG: refusal rule " + textbuf.StringUint8(uint8(r))
}

// layerRefusal is an image refused by a rule of the application table. It
// names the entry, when one entry broke the rule, and what the entry held.
// The fetch worker reports it under doctor-container-image-refused.
type layerRefusal struct {
	// entry is the entry's name, quoted and cut for display. It is empty for
	// a rule of the whole layer or image.
	entry string
	rule  refusalRule
	// detail is what broke the rule: an attribute name, a type, a bound.
	detail string
}

// Error answers the entry, the rule and the detail.
func (e *layerRefusal) Error() string {
	var text strings.Builder
	text.WriteString("image refused: ")
	if e.entry != "" {
		text.WriteString("entry ")
		text.WriteString(e.entry)
		text.WriteString(": ")
	}
	text.WriteString(e.rule.String())
	if e.detail != "" {
		text.WriteString(" (")
		text.WriteString(e.detail)
		text.WriteString(")")
	}
	return text.String()
}

// refuse answers the refusal of entry, an archive name, by rule.
func refuse(entry string, rule refusalRule, detail string) error {
	return &layerRefusal{entry: displayName(entry), rule: rule, detail: detail}
}

// refuseLayer answers the refusal of a whole layer or image by rule.
func refuseLayer(rule refusalRule, detail string) error {
	return &layerRefusal{rule: rule, detail: detail}
}

// entryFailure is an entry the system refused to apply: a write that failed,
// an owner the user Ze runs as cannot give. It is not a rule of the table,
// so it is not a refusal of the image's content.
type entryFailure struct {
	entry string
	err   error
}

// Error answers the entry and the system's reason.
func (e *entryFailure) Error() string {
	return "layer entry " + e.entry + " could not be applied: " + e.err.Error()
}

// Unwrap answers the system's reason.
func (e *entryFailure) Unwrap() error { return e.err }

// failEntry answers the failure of entry, an archive name.
func failEntry(entry string, err error) error {
	return &entryFailure{entry: displayName(entry), err: err}
}

// displayName quotes a name of the image for an error: in ASCII, so no
// control byte reaches an operator's terminal, and cut, so no name fills a
// log line.
func displayName(name string) string {
	if len(name) <= entryDisplayMax {
		return strconv.QuoteToASCII(name)
	}
	return strconv.QuoteToASCII(name[:entryDisplayMax]) + "..."
}

// entryPath is an archive name that passed the path rules, held as its
// components. The root of the image is the path with no component. Only
// parseEntryPath builds one from a name.
type entryPath struct {
	components []string
}

// parseEntryPath checks an archive name against the path rules. A folder's
// name may end in one slash. A "." component names the folder it is in, so
// it is passed over: it leaves nothing. Every other rule refuses, and
// nothing is corrected.
func parseEntryPath(name string, folder bool) (entryPath, error) {
	if strings.Contains(name, "\x00") {
		return entryPath{}, refuse(name, ruleNULInName, "")
	}
	if len(name) > entryPathMax {
		return entryPath{}, refuse(name, rulePathLength, textbuf.StringInt(int64(len(name)))+" bytes")
	}
	if strings.HasPrefix(name, "/") {
		return entryPath{}, refuse(name, ruleAbsolutePath, "")
	}
	if folder {
		name = strings.TrimSuffix(name, "/")
	}
	components := make([]string, 0, strings.Count(name, "/")+1)
	for component := range strings.SplitSeq(name, "/") {
		switch component {
		case "":
			return entryPath{}, refuse(name, ruleEmptyComponent, "")
		case "..":
			return entryPath{}, refuse(name, ruleParentComponent, "")
		case ".":
			continue
		}
		if len(component) > entryNameMax {
			return entryPath{}, refuse(name, ruleNameLength, textbuf.StringInt(int64(len(component)))+" bytes")
		}
		components = append(components, component)
	}
	return entryPath{components: components}, nil
}

// isRoot reports whether the path names the root of the image.
func (p entryPath) isRoot() bool { return len(p.components) == 0 }

// base answers the last component, empty for the root.
func (p entryPath) base() string {
	if p.isRoot() {
		return ""
	}
	return p.components[len(p.components)-1]
}

// parent answers the folder that holds the path. The root's parent is the
// root.
func (p entryPath) parent() entryPath {
	if p.isRoot() {
		return p
	}
	return entryPath{components: p.components[:len(p.components)-1]}
}

// key answers the path as one text, the components joined by a slash. It
// is what a pathrs call resolves, and the root is "." for it.
func (p entryPath) key() string {
	if p.isRoot() {
		return "."
	}
	return strings.Join(p.components, "/")
}

// underDevices reports whether the path is inside the image's device folder.
// The folder itself is not inside it.
func (p entryPath) underDevices() bool {
	if len(p.components) < 2 {
		return false
	}
	return p.components[0] == devicesFolder
}

// entryAttribute is one extended attribute of an entry.
type entryAttribute struct {
	name  string
	value string
}

// entryAttributes answers the extended attributes an entry carries, in name
// order, each judged by the attribute rule.
func entryAttributes(entry string, records map[string]string) ([]entryAttribute, error) {
	var attributes []entryAttribute
	total := 0
	for key, value := range records {
		if strings.HasPrefix(key, paxLibarchiveAttributePrefix) {
			return nil, refuse(entry, ruleAttributeName, "an attribute in the libarchive form")
		}
		name, ok := strings.CutPrefix(key, paxAttributePrefix)
		if !ok {
			continue
		}
		total += len(name) + len(value)
		if total > entryAttributesMax {
			return nil, refuse(entry, ruleAttributeBytes, "")
		}
		if err := checkAttributeName(entry, name); err != nil {
			return nil, err
		}
		attributes = append(attributes, entryAttribute{name: name, value: value})
	}
	slices.SortFunc(attributes, func(a, b entryAttribute) int { return strings.Compare(a.name, b.name) })
	return attributes, nil
}

// checkAttributeName judges one attribute name: the file capability, the
// two access-list names and the user namespace are kept, an overlay control
// attribute refuses the image, and so does any other name.
func checkAttributeName(entry, name string) error {
	if strings.HasPrefix(name, attributeOverlayTrusted) {
		return refuse(entry, ruleOverlayAttribute, displayName(name))
	}
	if strings.HasPrefix(name, attributeOverlayUser) {
		return refuse(entry, ruleOverlayAttribute, displayName(name))
	}
	switch name {
	case attributeFileCapability, attributeACLAccess, attributeACLDefault:
		return nil
	}
	if len(name) <= len(attributeUserPrefix) {
		return refuse(entry, ruleAttributeName, displayName(name))
	}
	if strings.HasPrefix(name, attributeUserPrefix) {
		return nil
	}
	return refuse(entry, ruleAttributeName, displayName(name))
}

// entryOwner answers the numeric owner and group of an entry. Names are
// ignored: the numbers are what the files hold.
func entryOwner(entry string, uid, gid int) (int, int, error) {
	if uid < 0 {
		return 0, 0, refuse(entry, ruleOwner, "owner "+textbuf.StringInt(int64(uid)))
	}
	if int64(uid) > ownerMax {
		return 0, 0, refuse(entry, ruleOwner, "owner "+textbuf.StringInt(int64(uid)))
	}
	if gid < 0 {
		return 0, 0, refuse(entry, ruleOwner, "group "+textbuf.StringInt(int64(gid)))
	}
	if int64(gid) > ownerMax {
		return 0, 0, refuse(entry, ruleOwner, "group "+textbuf.StringInt(int64(gid)))
	}
	return uid, gid, nil
}

// checkLinkTarget judges the target of a symbolic link. The target is kept
// as written and not resolved now: a process in the container resolves it.
func checkLinkTarget(entry, target string) error {
	if target == "" {
		return refuse(entry, ruleLinkTarget, "an empty target")
	}
	if len(target) > entryPathMax {
		return refuse(entry, ruleLinkTarget, textbuf.StringInt(int64(len(target)))+" bytes")
	}
	if strings.Contains(target, "\x00") {
		return refuse(entry, ruleLinkTarget, "a NUL byte")
	}
	return nil
}

// openLayerStream answers the decompressed stream of one layer blob. The
// caller MUST close it. Only the three accepted types are read: Ze does not
// guess a format.
func openLayerStream(blob io.Reader, compression registry.Compression) (io.ReadCloser, error) {
	switch compression {
	case registry.CompressionNone:
		return io.NopCloser(blob), nil
	case registry.CompressionGzip:
		stream, err := gzip.NewReader(blob)
		if err != nil {
			return nil, refuseLayer(ruleCompression, "gzip: "+err.Error())
		}
		return stream, nil
	case registry.CompressionZstd:
		// Concurrency 1 decodes in the caller's goroutine and starts none.
		decoder, err := zstd.NewReader(blob,
			zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderLowmem(true),
			zstd.WithDecoderMaxWindow(zstdWindowMax),
		)
		if err != nil {
			return nil, refuseLayer(ruleCompression, "zstd: "+err.Error())
		}
		return decoder.IOReadCloser(), nil
	case registry.CompressionUnspecified:
		return nil, refuseLayer(ruleCompression, "BUG: a layer with no type")
	}
	return nil, refuseLayer(ruleCompression, "BUG: layer type "+textbuf.StringUint8(uint8(compression)))
}

// expandedStream counts the decompressed bytes of a layer into the image's
// total, and refuses the first byte past the largest image size. The
// refusal is kept, because the archive reader above may answer its own
// error for a read that failed.
type expandedStream struct {
	stream  io.Reader
	counts  *applyCounts
	max     uint64
	refusal error
}

// Read reads from the stream and counts what it read.
func (s *expandedStream) Read(buffer []byte) (int, error) {
	if s.refusal != nil {
		return 0, s.refusal
	}
	n, err := s.stream.Read(buffer)
	s.counts.expandedBytes += uint64(n) //nolint:gosec // G115: a read count is not negative
	if s.counts.expandedBytes > s.max {
		s.refusal = refuseLayer(ruleExpandedBytes, "more than "+textbuf.StringUint(s.max)+" bytes")
		return n, s.refusal
	}
	return n, err
}
