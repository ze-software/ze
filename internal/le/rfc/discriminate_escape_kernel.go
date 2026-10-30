// Design: docs/contributing/rfc-conformance-gates.md -- the escape, and the precondition behind each reason
// Overview: discriminate_escape.go -- escapeCheck and the verdict this file answers one reason of
// Detail: rfc.go -- escapeKernel and the later route it names
//
// The kernel-producer escape covers a requirement Linux meets from the packet
// alone: a TTL the forwarding path checks, an ICMP error the kernel emits. No
// state Ze installs decides the outcome, so no break of Ze's own code can make
// the tagged unit red, and the row is published as escaped, which is debt.
// A break of the kernel could make it red, and the record names that route
// (patched-kernel) so the row stays owed until the route exists.
package rfc

import (
	"go/build"
	"io"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// kernelCitationWordsMin is the fewest words a cited claim span holds. A single
// word such as "TTL" sits in many claims, so it ties the escape to none.
const kernelCitationWordsMin = 3

// kernelExclusionWordsMin is the fewest words a ze-state-excluded reason
// holds. "does not decide the outcome" is five, and says nothing about which
// state the producer installs or why the claim does not rest on it.
const kernelExclusionWordsMin = 8

// kernelTestTree is the tree of test machinery a kernel-producer unit may
// import without an exclusion: it installs nothing on the product's behalf.
const kernelTestTree = "internal/test/"

// ZeStateExclusion is one Ze producer a kernel-producer unit reaches, and why
// the state it installs does not decide the outcome the claim names.
type ZeStateExclusion struct {
	// Producer is the Ze code, as <file>::<symbol> or a file alone, in a
	// package the unit's file imports.
	Producer string `json:"producer"`
	// Reason names the producer and says what state it installs and why the
	// claim's outcome does not rest on it.
	Reason string `json:"reason"`
}

// kernelOtherGOOS are the systems a kernel-producer unit MUST NOT build for.
// The claim is about the Linux kernel, so a file another system compiles runs
// somewhere that kernel is absent, and the escape would describe a test that
// observes some other stack.
var kernelOtherGOOS = []string{"darwin", "freebsd", "netbsd", "openbsd", "windows"}

// kernel checks the escape that names a Linux function instead of a producer.
//
// Five facts, each one a refusal of its own so the message names the one that
// failed. The carrier is a Go unit, because only a unit's run is the kernel's
// behavior observed directly. Its file builds only for Linux. Every Ze package
// that file imports is answered by an exclusion, because Ze code the unit runs
// can install the state that decides the outcome. The RFC summary declares a
// lower layer and its reason names each of the record's kernel functions, so
// the summary and the record agree on whose code answers. And the record cites
// a span of the tag's own claim, which claimSHA has already pinned against
// rewording: that is the tie to THIS claim.
func (e *escapeCheck) kernel() DiscriminationVerdict {
	if detail, refused := e.kernelCarrier(); refused {
		return e.unfounded(detail)
	}
	if detail, refused := e.kernelZeState(); refused {
		return e.unfounded(detail)
	}
	if detail, refused := e.kernelSummary(); refused {
		return e.unfounded(detail)
	}
	if detail, refused := e.kernelClaimTie(); refused {
		return e.unfounded(detail)
	}
	return DiscriminationVerdict{Record: e.record, State: ProofVerified}
}

// kernelCarrier refuses a carrier that is not a Linux-only Go unit.
func (e *escapeCheck) kernelCarrier() (string, bool) {
	unitFile := keyFile(e.record.Unit)
	var tb textbuf.Buffer
	tb.Str("the escape is ").Str(escapeKernel).Str(", which claims the Linux kernel produces ").
		Str("the behavior, but ")
	if !e.carried || e.carrier.Kind != kindUnit {
		return tb.Str(unitFile).Str(" is a ").Str(carrierKindOf(e.carrier, e.carried)).
			Str(" carrier. Only a Go unit test observes the kernel directly; a .ci or an ").
			Str("interop scenario runs Ze's own code, which a proof route can break").String(), true
	}
	only, err := buildsOnlyForLinux(e.reader, unitFile)
	if err != nil {
		return tb.Str(unitFile).Str(" has build constraints this gate cannot read: ").
			Err(err).String(), true
	}
	if !only {
		return tb.Str(unitFile).Str(" also builds for a system other than Linux, so it ").
			Str("observes a stack that is not the Linux kernel. Constrain it with ").
			Str("`//go:build linux` or a _linux_test.go name, or take a proof route").String(), true
	}
	return "", false
}

// buildsOnlyForLinux answers whether go/build compiles the file for Linux and
// for none of kernelOtherGOOS.
//
// go/build reads the constraints the way the go command does, the
// `//go:build` line and the _GOOS file-name suffix, which is how
// unitNeedsGuest already asks the question. The integration tag is set,
// because a test that touches the kernel carries `integration && linux`
// (ai/rules/platform-linux.md). The file is read through the gate's own reader
// so a fixture tree answers what the checkout answers.
func buildsOnlyForLinux(reader *sourceReader, file string) (bool, error) {
	content := reader.read(file)
	if content == nil {
		var tb textbuf.Buffer
		return false, parseErr(tb.Str(file).Str(" is not in this tree"))
	}
	match := func(goos string) (bool, error) {
		context := build.Default
		context.GOOS = goos
		context.CgoEnabled = false
		context.BuildTags = []string{testqemu.IntegrationTag}
		context.OpenFile = func(string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(*content)), nil
		}
		directory, name := filepath.Split(file)
		return context.MatchFile(directory, name)
	}
	onLinux, err := match(guestGOOS)
	if err != nil {
		return false, err
	}
	if !onLinux {
		return false, nil
	}
	for _, goos := range kernelOtherGOOS {
		elsewhere, err := match(goos)
		if err != nil {
			return false, err
		}
		if elsewhere {
			return false, nil
		}
	}
	return true, nil
}

// kernelZeState refuses a unit that reaches Ze code no exclusion answers for.
//
// The escape says no state Ze installs decides the outcome. A unit that
// imports a Ze package runs its code, and the state that code installs (a
// route, an MTU, an address) can decide whether the packet ever meets the
// kernel function: a datagram that forwards over a route Ze installed is Ze's
// outcome, and the mutant route against that producer is how it is proved. So
// each Ze package the unit's file imports, test machinery under kernelTestTree
// apart, is named by an exclusion whose producer sits in it, and every
// exclusion names a producer this tree holds in a package the file imports.
//
// Every imported Ze package is asked, not only the ones gomu mutates. Which
// ones gomu mutates is a walk of each package's directory, which the reader
// does not hold, and asking one package too many costs the author one
// exclusion, while asking one too few is the blanket opt-out this check closes.
// The reason's shape (its length, the producer it names) is checked at load
// time by validateKernelEscape; whether it is TRUE is a judge's question.
func (e *escapeCheck) kernelZeState() (string, bool) {
	unitFile := keyFile(e.record.Unit)
	var tb textbuf.Buffer
	tb.Str("the escape is ").Str(escapeKernel).
		Str(", which claims no state Ze installs decides the outcome, but ")
	module, held := moduleOf(e.reader)
	if !held {
		return tb.Str("go.mod names no module, so this gate cannot tell which imports of ").
			Str(unitFile).Str(" are Ze's own code").String(), true
	}
	content := e.reader.read(unitFile)
	if content == nil {
		return tb.Str(unitFile).Str(" is not in this tree").String(), true
	}
	imported, err := goImportPaths(*content, unitFile)
	if err != nil {
		return tb.Str(unitFile).Str(" does not parse, so its imports cannot be read: ").
			Err(err).String(), true
	}
	reached := kernelZePackages(imported, module)
	for index := range e.record.ZeStateExcluded {
		producer := e.record.ZeStateExcluded[index].Producer
		directory := path.Dir(keyFile(producer))
		if !slices.Contains(reached, directory) {
			return tb.Str("ze-state-excluded names ").Quoted(producer).Str(" in ").Str(directory).
				Str(", a package ").Str(unitFile).Str(" does not import. Name the producers of ").
				Str("the Ze packages the unit imports: ").Join(reached, ", ").String(), true
		}
		_, found, err := resolveKeyText(e.reader.read, e.index, producer, e.record.Source)
		if err != nil || !found {
			return tb.Str("ze-state-excluded names ").Quoted(producer).
				Str(", and this tree holds no such code. Name a function that file declares, ").
				Str("or the file alone").String(), true
		}
	}
	for _, directory := range reached {
		if !slices.ContainsFunc(e.record.ZeStateExcluded, func(entry ZeStateExclusion) bool {
			return path.Dir(keyFile(entry.Producer)) == directory
		}) {
			return tb.Str(unitFile).Str(" imports ").Str(directory).
				Str(", Ze code that can install the state deciding the outcome. Where it does, ").
				Str("take the ").Str(RouteMutant).Str(" route against that producer; where it ").
				Str("does not, name the producer in ze-state-excluded with the reason").String(), true
		}
	}
	return "", false
}

// kernelZePackages answers the repository-relative directories of the Ze
// packages among a file's imports, test machinery apart, each once.
func kernelZePackages(imported []string, module string) []string {
	var out []string
	for _, importPath := range imported {
		relative, inModule := strings.CutPrefix(importPath, module+"/")
		if !inModule {
			continue
		}
		if strings.HasPrefix(relative, kernelTestTree) {
			continue
		}
		if !slices.Contains(out, relative) {
			out = append(out, relative)
		}
	}
	return out
}

// moduleOf answers the module path go.mod declares, read through the gate's
// own reader so a fixture tree answers what the checkout answers.
func moduleOf(reader *sourceReader) (string, bool) {
	content := reader.read("go.mod")
	if content == nil {
		return "", false
	}
	return moduleLine(*content)
}

// kernelSummary refuses an escape whose RFC summary does not say a lower layer
// performs part of the document, or whose reason does not name each of the
// record's kernel functions.
//
// The summary is where the repository declares whose code answers the RFC, and
// the public page reads it. An escape saying "Linux produces this" over a
// summary saying "Ze's Go answers it" states two things that cannot both be
// true, and a kernel function the summary never names is an implementer only
// the record knows about.
func (e *escapeCheck) kernelSummary() (string, bool) {
	var tb textbuf.Buffer
	tb.Str("the escape is ").Str(escapeKernel).Str(" naming ").Join(e.record.KernelFunctions, ", ").
		Str(", but ")
	located, held := evidenceAt(discriminationRel, e.record.Source)
	if !held {
		return tb.Str("the record was read from ").Quoted(e.record.Source).
			Str(", which names no RFC, so no summary can be read for it").String(), true
	}
	summary := path.Join(summaryRel, located.Stem+".md")
	text := e.reader.read(summary)
	if text == nil {
		return tb.Str(summary).Str(" is not in this tree").String(), true
	}
	meta, err := ParseMeta(*text, located.Stem, summary)
	if err != nil {
		return tb.Str(summary).Str(" does not parse: ").Err(err).String(), true
	}
	if !implementationNamesAnImplementer(meta.Implementation) {
		return tb.Str(summary).Str(" declares its Implementation ").Quoted(meta.Implementation).
			Str(". A kernel producer is a lower layer, so the summary declares ").
			Str(implementationMixed).Str(" or ").Str(implementationThirdParty).
			Str(" and names it, or the row takes a proof route").String(), true
	}
	for _, function := range e.record.KernelFunctions {
		if !namesToken(meta.ImplementationReason, function) {
			return tb.Str("the Implementation reason of ").Str(summary).Str(" does not name ").
				Str(function).Str(". Name each kernel function there in the same ").
				Str("<path>::<function> spelling, so the summary and the record agree on whose ").
				Str("code answers").String(), true
		}
	}
	return "", false
}

// namesToken answers whether text names name as a whole token: no path or
// identifier character before it and no identifier character after it, so
// ip_forward is not found inside ip_forward_finish.
//
// Each pass moves offset past the previous hit, so the loop ends within
// len(text) passes.
func namesToken(text, name string) bool {
	if name == "" {
		return false
	}
	for offset := 0; offset < len(text); {
		found := strings.Index(text[offset:], name)
		if found < 0 {
			return false
		}
		start := offset + found
		end := start + len(name)
		before := start > 0 && (identifierByte(text[start-1]) || strings.IndexByte("/.", text[start-1]) >= 0)
		after := end < len(text) && identifierByte(text[end])
		if !before && !after {
			return true
		}
		offset = start + 1
	}
	return false
}

// identifierByte answers whether b can sit inside a C or Go identifier.
func identifierByte(b byte) bool {
	return b == '_' || ('0' <= b && b <= '9') || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

// kernelClaimTie refuses a citation that is not a span of the tag's own claim.
//
// Whitespace is collapsed on both sides, as claimSHA collapses it, so a claim
// re-wrapped across comment lines still contains the span it said.
func (e *escapeCheck) kernelClaimTie() (string, bool) {
	cited := strings.Join(strings.Fields(e.record.Citation), " ")
	var tb textbuf.Buffer
	tb.Str("the escape is ").Str(escapeKernel).Str(" citing ").Quoted(e.record.Citation)
	if len(strings.Fields(cited)) < kernelCitationWordsMin {
		return tb.Str(", fewer than ").Int(int64(kernelCitationWordsMin)).
			Str(" words. A short span sits in many claims and ties the escape to none: cite ").
			Str("the words that state the kernel's behavior").String(), true
	}
	for index := range e.tagged {
		claim := strings.Join(strings.Fields(e.tagged[index].Claim), " ")
		if strings.Contains(claim, cited) {
			return "", false
		}
	}
	return tb.Str(", which the tag's claim does not say. The citation ties the escape to ").
		Str("THIS claim, so it is copied from the claim's own words").String(), true
}
