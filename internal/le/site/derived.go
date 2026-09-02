// Design: website/AI.md -- llms.txt is the one denormalized answer for a machine reader
// Detail: llmsdata.go loads the inputs; catalog.go the live commands; docsmanifest.go the page list.
// Related: commands.go and equivalents.go publish the same commands as pages.
package site

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// llms.txt is registered from here, so a build discovers it through the
// registry rather than through a call the build states by name.
//
// It answers NO route: it is a published file, not a page, so the coverage
// arithmetic must not count it. AC-16 gives the named non-route artifacts their
// own check in phase 10.
func init() {
	registerProducer(Producer{Name: "llms", Render: renderLLMS})
}

// llmsFile is the published path of the denormalized machine-reader answer.
const llmsFile = "llms.txt"

// renderLLMS publishes llms.txt.
//
// The file is ONE producer's output. Before this, internal/le/docvalid wrote it
// from the command catalog alone, which cut it from 1035 lines to 399 and lost
// seventeen of its eighteen sections. A second writer for one path means the
// last writer wins and nothing says which one ran.
func renderLLMS(paths Paths) ([]string, error) {
	inputs, err := loadLLMSInputs(paths)
	if err != nil {
		return nil, err
	}
	var out textbuf.Buffer
	writeLLMSIntro(&out)
	writeLLMSProductSnapshot(&out, inputs)
	writeLLMSQualityModel(&out, inputs)
	writeLLMSComparison(&out)
	writeLLMSFeatures(&out, inputs)
	writeLLMSConfigRoots(&out, inputs)
	writeLLMSPlugins(&out, inputs)
	writeLLMSCommands(&out, inputs)
	writeLLMSEquivalents(&out, inputs)
	writeLLMSDependencies(&out, inputs)
	if err := writeLLMSDocumentation(&out, paths); err != nil {
		return nil, err
	}
	writeLLMSPageMap(&out, inputs)

	content := strings.TrimRight(out.String(), "\n") + "\n"
	path := filepath.Join(paths.Output, llmsFile)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return nil, err
	}
	return nil, nil
}

// writeLLMSIntro states what this file is and how to read it.
func writeLLMSIntro(out *textbuf.Buffer) {
	out.Str("# Ze\n\n")
	out.Str("> Ze is an open-source configuration and protocol engine. The network operating ").
		Str("system built on it speaks BGP, manages Linux interfaces, programs the FIB, and serves the ").
		Str("same YANG-modeled configuration through CLI, SSH, web, API, and MCP. Its core holds the ").
		Str("supervisor, message bus, config provider, and plugin manager; protocols and services arrive ").
		Str("as subsystems or plugins.\n\n")
	out.Str("Pre-release: no tagged versions yet, built continuously from the main branch. ").
		Str("AGPLv3 open source. See the ExaBGP [migration path](use-cases/exabgp-migration/index.md).\n\n")
	out.Str("This file is intentionally denormalized for AI use. It includes the high-signal ").
		Str("product inventory and then the normal page map, so common questions should not require ").
		Str("fetching many separate pages. Page links still point at Markdown `index.md` files first, ").
		Str("with rendered web URLs beside them for humans.\n\n")
}

// writeLLMSProductSnapshot states what Ze is, in numbers this build derived.
func writeLLMSProductSnapshot(out *textbuf.Buffer, inputs *llmsInputs) {
	facts := inputs.Facts
	out.Str("## Product snapshot\n\n")
	out.Str("- Purpose: configuration and protocol engine for Linux routing, plus a network ").
		Str("operating system built on that core.\n")
	out.Str("- Protocols and subsystems in the shipped daemon: BGP, IS-IS, OSPF, BFD, static ").
		Str("routes, policy routing, FIB programming, interfaces, firewall, traffic control, DNS, DHCP, ").
		Str("NTP, IPsec, L2TP, PPPoE, telemetry, web UI, SSH CLI, MCP, and plugins.\n")
	out.Str("- Operator surfaces: SSH CLI with commit and rollback, generated command ").
		Str("reference, server-rendered web workbench, looking glass, telemetry, gNMI, gRPC, MCP, ").
		Str("JSON/YAML/NDJSON/table output, and shell-like output pipes derived from the schema where ").
		Str("possible.\n")
	out.Str("- Dataplane: Linux netlink, nftables, eBPF, AF_PACKET, psample, optional VPP ").
		Str("integrations, and namespace-aware testing.\n")
	out.Str("- Release state: pre-release, main-branch builds, no tagged stable release yet.\n")
	out.Str("- License and repos: AGPLv3. Canonical repository: ").Str(repositoryURL).Str(". Discord: ").
		Str(discordInvite).Str(".\n")
	out.Str("- Current generated counts: ").Int(int64(facts.Features.CoreExperimental)).
		Str(" shipped or experimental feature cards, ").Int(int64(facts.Features.Planned)).Str(" roadmap cards, ").
		Int(int64(facts.CLICommands)).Str(" CLI commands, ").Int(int64(facts.ConfigSections)).
		Str(" config sections, ").Int(int64(len(inputs.Plugins))).Str(" plugin registrations, ").
		Int(int64(facts.Dependencies)).Str(" direct Go dependencies, ").Int(int64(facts.Changes)).
		Str(" weekly change entries.\n")
	out.Str("- Test evidence counts: ").Str(facts.Tests.UnitDisplay).Str(" unit tests, ").Str(facts.Tests.FuzzDisplay).
		Str(" fuzz targets, ").Str(facts.Tests.E2EDisplay).Str(" end-to-end transcript steps, ").
		Int(int64(facts.Interop.Scenarios)).Str(" interop scenarios across ").Str(facts.Interop.TargetDisplay).
		Str(" target implementations.\n")
	out.Str("- Generated date: ").Str(facts.GeneratedAt).Str(".\n\n")
}

// writeLLMSQualityModel states how Ze proves what it claims, and with which
// commands a reader can re-run each layer.
//
// Every command here is an action this repository registers today. The retired
// renderer named eight `make` targets, and `make` went with the interpreter
// cutover, so a reader who copied that paragraph got "no rule to make target".
func writeLLMSQualityModel(out *textbuf.Buffer, inputs *llmsInputs) {
	tests := inputs.Facts.Tests
	out.Str("## Quality and verification model\n\n")
	out.Str("Ze uses layered proof because bugs appear at different boundaries.\n\n")
	out.Str("- Local Go tests: package behavior, parser rules, encoders, state transitions, ").
		Str("validation paths, and error shapes. Current scale: ").Str(tests.UnitDisplay).Str(" unit tests.\n")
	out.Str("- Race, coverage, and fuzz: fuzz targets are normal Go tests with generated ").
		Str("input. Current scale: ").Str(tests.FuzzDisplay).Str(" fuzz targets.\n")
	out.Str("- gomu mutation checks: mutate production Go code and rerun tests to find weak ").
		Str("assertions. gomu is advisory, not the default CI gate.\n")
	out.Str("- Functional `.ci` transcripts: drive processes, CLI commands, files, HTTP, ").
		Str("syslog, peers, daemons, exits, and BGP wire expectations. BGP failures are decoded ").
		Str("structurally, not shown as raw hex only.\n")
	out.Str("- Browser `.wb` transcripts: drive the rendered web UI through real browser flows.\n")
	out.Str("- Editor `.et` transcripts: drive the headless interactive editor.\n")
	out.Str("- QEMU: runs Linux-only behavior from macOS or CI where netlink, nftables, eBPF, ").
		Str("PPP, network namespaces, and kernel modules exist.\n")
	out.Str("- Interop: ").Int(int64(inputs.Facts.Interop.Scenarios)).Str(" scenarios against ").
		Str(inputs.Facts.Interop.TargetDisplay).Str(" target implementations, including FRR, BIRD, GoBGP, ").
		Str("RustyBGP, OpenBGPD, ExaBGP, and other real daemons where applicable.\n")
	out.Str("- Verify workflow: `./le verify worktree` runs the whole native verification ").
		Str("population against a fixed commit in a detached worktree, writes stage logs under `tmp/`, ").
		Str("groups related failures, and prints narrow rerun commands. `./le repository` is the ").
		Str("narrower handoff gate.\n")
	out.Str("- Rule for regressions: do not hide a failure with a skip or loose assertion. ").
		Str("Move the proof to the layer that can see the real behavior, add the narrow test, rerun it, ").
		Str("then rerun the gate that should have caught it.\n\n")
	out.Str("Useful commands: `go test -race -run TestName ./internal/...`, `./le fuzz run`, ").
		Str("`gomu run`, `bin/ze-test bgp plugin 42 -v`, `./le qemu netns-test`, ").
		Str("`./le integration interop`, `./le evidence release-candidate`.\n\n")
}

// writeLLMSComparison states which daemons Ze is compared with, and on what
// evidence a comparison claim rests.
func writeLLMSComparison(out *textbuf.Buffer) {
	out.Str("## Comparison positioning\n\n")
	out.Str("- BGP comparison lens: Ze is compared with BIRD, FRR, OpenBGPD, GoBGP, bio-rd, ").
		Str("ExaBGP, RustyBGP, rustbgpd, and freeRtr across AFI/SAFI, core protocol, policy, security, ").
		Str("observability, APIs, operations, and best-path behavior.\n")
	out.Str("- Network OS lens: Ze is compared with VyOS and freeRtr across routing, ").
		Str("interfaces, firewall, NAT, VPN, AAA, services, management APIs, automation, packaging, ").
		Str("observability, tests, and implementation model.\n")
	out.Str("- Evidence policy: capability claims should cite upstream code, official feature ").
		Str("documentation, or the integration layer that owns the behavior. `Unclear`, `Partial`, and ").
		Str("`Not found` are valid outcomes when evidence does not support a stronger claim.\n")
	out.Str("- Comparison pages are advice for product decisions, not marketing copy.\n\n")
}

// writeLLMSFeatures lists every feature card in the order website/data states.
func writeLLMSFeatures(out *textbuf.Buffer, inputs *llmsInputs) {
	out.Str("## Feature inventory\n\n")
	for index := range inputs.Features.Sections {
		section := &inputs.Features.Sections[index]
		counts := make(map[string]int, 4)
		for _, card := range section.Cards {
			counts[cardStatus(card.Status)]++
		}
		summary := make([]string, 0, len(counts))
		for _, status := range sortedStatuses(counts) {
			summary = append(summary, strconv.Itoa(counts[status])+" "+status)
		}
		out.Str("### ").Str(cleanInline(section.Heading)).Str(" (").Int(int64(len(section.Cards))).Str(" cards: ").
			Str(strings.Join(summary, ", ")).Str(")\n")
		if lead := cleanInline(section.Lead); lead != "" {
			out.Str(lead).Byte('\n')
		}
		for cardIndex := range section.Cards {
			out.Str(featureCardLine(&section.Cards[cardIndex])).Byte('\n')
		}
		out.Byte('\n')
	}
}

// cardStatus answers a card's status, defaulting to the one a card with none
// carries: a card that ships is the ordinary case and states nothing.
func cardStatus(status string) string {
	if status == "" {
		return "current"
	}
	return status
}

// sortedStatuses answers the statuses of one section in ascending order, so the
// count line is the same on every build.
func sortedStatuses(counts map[string]int) []string {
	statuses := make([]string, 0, len(counts))
	for status := range counts {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	return statuses
}

// featureCardLine writes one feature card as one line.
func featureCardLine(card *featureCard) string {
	line := "- " + cleanInline(card.Title) + " [" + cleanInline(orUncategorized(card.Category)) +
		", " + cardStatus(card.Status) + "]"
	var parts []string
	if chips := cardChipNames(card); len(chips) != 0 {
		parts = append(parts, "chips: "+strings.Join(chips, ", "))
	}
	bullets := make([]string, 0, len(card.Bullets))
	for _, bullet := range card.Bullets {
		if cleaned := cleanInline(bullet); cleaned != "" {
			bullets = append(bullets, cleaned)
		}
	}
	if len(bullets) != 0 {
		parts = append(parts, strings.Join(bullets, "; "))
	}
	if card.Href != "" {
		parts = append(parts, "link: "+mirrorURL(card.Href))
	}
	if len(parts) == 0 {
		return line
	}
	return line + ": " + strings.Join(parts, "; ")
}

// cardChipNames answers the chip labels of one card, dropping the empty ones.
func cardChipNames(card *featureCard) []string {
	names := make([]string, 0, len(card.Chips))
	for _, chip := range card.Chips {
		if cleaned := cleanInline(chip.Text); cleaned != "" {
			names = append(names, cleaned)
		}
	}
	return names
}

// orUncategorized answers a category, or says the card declared none.
func orUncategorized(value string) string {
	if value == "" {
		return "uncategorized"
	}
	return value
}

// writeLLMSConfigRoots lists the top-level YANG config roots and their direct
// children, which is enough to orient without fetching the full reference.
func writeLLMSConfigRoots(out *textbuf.Buffer, inputs *llmsInputs) {
	out.Str("## Configuration model roots\n\n")
	out.Str("Top-level YANG-derived config roots. Child names are direct children only, ").
		Str("enough to orient without fetching the full reference.\n\n")
	names := make([]string, 0, len(inputs.ConfigTree))
	for name := range inputs.ConfigTree {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		node := inputs.ConfigTree[name]
		description := trimInline(node.Description, 180)
		if description == "" {
			description = cleanInline(orConfigRoot(node.Kind))
		}
		children := make([]string, 0, len(node.Children))
		for _, child := range node.Children {
			children = append(children, cleanInline(child.Name))
		}
		listed := ""
		if len(children) != 0 {
			listed = strings.Join(children[:minimum(len(children), configChildrenListed)], ", ")
			if len(children) > configChildrenListed {
				listed += ", ..."
			}
		}
		listed = orNone(listed)
		out.Str("- `").Str(name).Str("`: ").Str(description).Str(" Children: ").Str(listed).Str(".\n")
	}
	out.Byte('\n')
}

// configChildrenListed bounds the children one root names, so a root with sixty
// leaves does not push the rest of the file out of a reader's window.
const configChildrenListed = 14

// minimum answers the smaller of two counts.
func minimum(left, right int) int {
	if left < right {
		return left
	}
	return right
}

// orConfigRoot answers a node's kind, or names it a config root.
func orConfigRoot(value string) string {
	if value == "" {
		return "config root"
	}
	return value
}

// writeLLMSPlugins lists every runtime plugin registration.
func writeLLMSPlugins(out *textbuf.Buffer, inputs *llmsInputs) {
	out.Str("## Plugin registry\n\n")
	out.Str("Each registration comes from the Go runtime registry. Config roots come from ").
		Str("plugin metadata and YANG files.\n\n")
	plugins := make([]registryPlugin, len(inputs.Plugins))
	copy(plugins, inputs.Plugins)
	sort.Slice(plugins, func(left, right int) bool { return plugins[left].Name < plugins[right].Name })
	for index := range plugins {
		plugin := &plugins[index]
		out.Str("- `").Str(cleanInline(plugin.Name)).Str("`: ").Str(trimInline(plugin.Description, 170)).
			Str(" Config roots: ").Str(joinOrNone(plugin.ConfigRoots)).Str(". Dependencies: ").
			Str(joinOrNone(plugin.Dependencies)).Str(". Optional: ").Str(joinOrNone(plugin.OptionalDependencies)).
			Str(". YANG files: ").Int(int64(len(plugin.YangFiles))).Str(". Source: `").
			Str(cleanInline(plugin.SourceDir)).Str("`.\n")
	}
	out.Byte('\n')
}

// joinOrNone writes a list for a reader, saying "none" rather than leaving the
// sentence to end on a colon.
func joinOrNone(values []string) string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := cleanInline(value); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return orNone(strings.Join(cleaned, ", "))
}

// writeLLMSCommands lists every live command, grouped by its verb.
//
// The section heading is `## CLI command surface` and each command is one `- `
// line opening with its path in a code span. Both are read back by the
// documentation drift check, which extracts this section by heading and each
// command's metadata from its line.
func writeLLMSCommands(out *textbuf.Buffer, inputs *llmsInputs) {
	commands := inputs.Commands
	modes := make(map[string]int, 4)
	byVerb := make(map[string][]*catalogCommand, 32)
	var verbs []string
	for index := range commands {
		command := &commands[index]
		modes[command.Mode]++
		verb, _, _ := strings.Cut(command.Path, " ")
		if _, seen := byVerb[verb]; !seen {
			verbs = append(verbs, verb)
		}
		byVerb[verb] = append(byVerb[verb], command)
	}
	sort.Strings(verbs)

	modeNames := make([]string, 0, len(modes))
	for mode := range modes {
		modeNames = append(modeNames, mode)
	}
	sort.Strings(modeNames)
	counted := make([]string, 0, len(modeNames))
	for _, mode := range modeNames {
		counted = append(counted, strconv.Itoa(modes[mode])+" "+mode)
	}

	out.Str("## CLI command surface\n\n")
	out.Str("The command catalog is generated from `ze help command --json`, not ").Str("hand-written. Modes: ").
		Str(strings.Join(counted, ", ")).Str(".\n")
	out.Str("`daemon` commands require a running Ze daemon. `read-only` commands query ").
		Str("state. `offline` commands can run without daemon state. `pipes` groups operators as ").
		Str("`always`, `with-rows`, `when-streaming`, or `local-only`; these qualifiers are part of ").
		Str("the contract. An operator absent from those groups is refused by name.\n\n")
	for _, verb := range verbs {
		group := byVerb[verb]
		out.Str("### `").Str(verb).Str("` commands (").Int(int64(len(group))).Str(")\n")
		for _, command := range group {
			// The whole summary, with no character budget: it is declared as
			// one line, so there is nothing left for a cut to do but stop a
			// sentence mid-clause.
			out.Str("- `").Str(command.Path).Str("` (").Str(commandMetadataLine(command)).Str("): ").
				Str(cleanInline(command.Description)).Byte('\n')
		}
		out.Byte('\n')
	}
}

// commandMetadataLine writes one command's whole contract as one clause.
func commandMetadataLine(command *catalogCommand) string {
	meta := []string{command.Mode}
	if command.WireMethod != "" {
		meta = append(meta, "wire "+command.WireMethod)
	}
	grouped := operatorsByAvailability(command)
	var pipeGroups []string
	for _, availability := range availabilityOrder {
		if names := grouped[availability]; len(names) != 0 {
			pipeGroups = append(pipeGroups, availability+": "+strings.Join(names, " "))
		}
	}
	if len(pipeGroups) != 0 {
		meta = append(meta, "pipes "+strings.Join(pipeGroups, ", "))
	}
	if command.AnswerShape != "" {
		meta = append(meta, "shape "+command.AnswerShape)
	}
	if len(command.AddressFields) != 0 {
		meta = append(meta, "address-fields "+strings.Join(command.AddressFields, " "))
	}
	if len(command.Pipes) != 0 {
		names := make([]string, 0, len(command.Pipes))
		for _, pipe := range command.Pipes {
			names = append(names, "`"+pipe.Name+"`")
		}
		meta = append(meta, "filters "+strings.Join(names, " "))
	}
	if len(command.Aliases) != 0 {
		values := make([]string, 0, len(command.Aliases))
		for _, alias := range command.Aliases {
			values = append(values, "`"+alias.Name+"`=`"+alias.Expansion+"`")
		}
		meta = append(meta, "aliases "+strings.Join(values, ", "))
	}
	if len(command.Args) != 0 {
		values := make([]string, 0, len(command.Args))
		for _, argument := range command.Args {
			values = append(values, argument.Name+":"+argument.Type)
		}
		meta = append(meta, "args "+strings.Join(values, ", "))
	}
	if command.Usage != "" {
		meta = append(meta, "usage `"+command.Usage+"`")
	}
	return strings.Join(meta, "; ")
}

// writeLLMSEquivalents lists the curated vendor command map.
func writeLLMSEquivalents(out *textbuf.Buffer, inputs *llmsInputs) {
	mapping := inputs.Equivalents
	vendors := mapping.vendorIDs()
	out.Str("## Vendor command equivalents\n\n")
	out.Str(cleanInline(mapping.Summary)).Byte('\n')
	out.Str("Updated: ").Str(cleanInline(mapping.Updated)).Str(".\n")
	rooted := make([]string, 0, len(vendors))
	for _, vendor := range vendors {
		rooted = append(rooted, mapping.vendorLabel(vendor)+" ("+mapping.Vendors[vendor].RootingModel+")")
	}
	out.Str("Vendors: ").Str(strings.Join(rooted, ", ")).Str(".\n\n")
	for index := range mapping.Entries {
		entry := &mapping.Entries[index]
		parts := []string{"- " + cleanInline(entry.Category) + ": " + cleanInline(entry.Intent)}
		ze := make([]string, 0, len(entry.Ze))
		for _, path := range entry.Ze {
			ze = append(ze, "`"+path+"`")
		}
		parts = append(parts, "Ze: "+strings.Join(ze, ", "))
		for _, vendor := range vendors {
			rows := entry.Vendors[vendor]
			if len(rows) == 0 {
				continue
			}
			lines := make([]string, 0, len(rows))
			for _, item := range rows {
				line := "`" + item.Command + "`"
				var meta []string
				if item.Mode != "" {
					meta = append(meta, item.Mode)
				}
				if item.Confidence != "" {
					meta = append(meta, item.Confidence)
				}
				if len(meta) != 0 {
					line += " (" + strings.Join(meta, ", ") + ")"
				}
				lines = append(lines, line)
			}
			parts = append(parts, mapping.vendorLabel(vendor)+": "+strings.Join(lines, "; "))
		}
		out.Str(strings.Join(parts, ". ")).Str(".\n")
	}
	out.Byte('\n')
}

// writeLLMSDependencies states why Ze takes each direct Go module.
func writeLLMSDependencies(out *textbuf.Buffer, inputs *llmsInputs) {
	out.Str("## Dependency rationale\n\n")
	out.Str("Direct Go modules are grouped by why Ze needs them. This is generated from ").
		Str("go.mod plus curated rationale, not copied from package names alone.\n\n")
	for index := range inputs.Dependencies.Categories {
		category := &inputs.Dependencies.Categories[index]
		out.Str("### ").Str(cleanInline(category.Name)).Str(" (").Int(int64(len(category.Modules))).Str(")\n")
		for _, module := range category.Modules {
			out.Str("- `").Str(cleanInline(module.Module)).Str("`: ").Str(trimInline(module.Why, 240)).Byte('\n')
		}
		out.Byte('\n')
	}
}

// writeLLMSPageMap lists the curated navigation, dropdown by dropdown.
func writeLLMSPageMap(out *textbuf.Buffer, inputs *llmsInputs) {
	out.Str("## Page map\n\n")
	out.Str("Every link points to the page Markdown mirror first. The web URL is the ").
		Str("human-rendered version of the same page.\n\n")
	for index := range inputs.Nav.Dropdowns {
		dropdown := &inputs.Nav.Dropdowns[index]
		out.Str("## ").Str(dropdown.Label).Str("\n\n")
		for _, column := range dropdown.Columns {
			for _, entry := range column {
				if entry.LabelOnly != "" {
					out.Str("### ").Str(entry.LabelOnly).Byte('\n')
					continue
				}
				out.Str(navEntryLine(entry, inputs))
			}
		}
		out.Byte('\n')
	}
	out.Str("## More\n\n")
	for _, link := range inputs.Nav.TrailingLinks {
		out.Str("- [").Str(link.Label).Str("](").Str(mirrorURL(link.Href)).Str(") (web: ").Str(pageURL(link.Href)).
			Str(")\n")
	}
	out.Str("- [Discord](").Str(discordInvite).Str("): community and support\n")
	out.Str("- [GitHub](").Str(repositoryURL).Str("): canonical repository, issues, wiki\n")
}

// navEntryLine writes one navigation entry, with the count-carrying description
// taken from the facts snapshot rather than from the menu's own prose.
//
// A menu description ages: "402 commands" is written once and read for a year.
// The four entries below name a number this build already derived, so the file
// states the current one.
func navEntryLine(entry navEntry, inputs *llmsInputs) string {
	description := cleanInline(entry.Desc)
	if fresh := liveNavDescription(entry.Href, inputs); fresh != "" {
		description = fresh
	}
	return "- [" + entry.Title + "](" + mirrorURL(entry.Href) + "): " + description +
		" (web: " + pageURL(entry.Href) + ")\n"
}

// liveNavDescription answers the description a navigation entry takes from this
// build's own numbers, or the empty string when the entry has none.
func liveNavDescription(href string, inputs *llmsInputs) string {
	facts := inputs.Facts
	switch href {
	case sectionFeatures:
		return strconv.Itoa(facts.Features.CoreExperimental) + " features, color-coded by category"
	case "reference/cli/":
		return strconv.Itoa(facts.CLICommands) + " commands, generated from the live binary"
	case "reference/dependencies/":
		return strconv.Itoa(facts.Dependencies) + " direct packages, generated from go.mod"
	case "project/changes/":
		return strconv.Itoa(facts.Changes) + " weekly updates, newest first"
	default:
		return ""
	}
}
