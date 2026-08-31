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
	var out strings.Builder
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
func writeLLMSIntro(out *strings.Builder) {
	out.WriteString("# Ze\n\n")
	out.WriteString("> Ze is an open-source configuration and protocol engine. The network operating " +
		"system built on it speaks BGP, manages Linux interfaces, programs the FIB, and serves the " +
		"same YANG-modeled configuration through CLI, SSH, web, API, and MCP. Its core holds the " +
		"supervisor, message bus, config provider, and plugin manager; protocols and services arrive " +
		"as subsystems or plugins.\n\n")
	out.WriteString("Pre-release: no tagged versions yet, built continuously from the main branch. " +
		"AGPLv3 open source. See the ExaBGP [migration path](use-cases/exabgp-migration/index.md).\n\n")
	out.WriteString("This file is intentionally denormalized for AI use. It includes the high-signal " +
		"product inventory and then the normal page map, so common questions should not require " +
		"fetching many separate pages. Page links still point at Markdown `index.md` files first, " +
		"with rendered web URLs beside them for humans.\n\n")
}

// writeLLMSProductSnapshot states what Ze is, in numbers this build derived.
func writeLLMSProductSnapshot(out *strings.Builder, inputs *llmsInputs) {
	facts := inputs.Facts
	out.WriteString("## Product snapshot\n\n")
	out.WriteString("- Purpose: configuration and protocol engine for Linux routing, plus a network " +
		"operating system built on that core.\n")
	out.WriteString("- Protocols and subsystems in the shipped daemon: BGP, IS-IS, OSPF, BFD, static " +
		"routes, policy routing, FIB programming, interfaces, firewall, traffic control, DNS, DHCP, " +
		"NTP, IPsec, L2TP, PPPoE, telemetry, web UI, SSH CLI, MCP, and plugins.\n")
	out.WriteString("- Operator surfaces: SSH CLI with commit and rollback, generated command " +
		"reference, server-rendered web workbench, looking glass, telemetry, gNMI, gRPC, MCP, " +
		"JSON/YAML/NDJSON/table output, and shell-like output pipes derived from the schema where " +
		"possible.\n")
	out.WriteString("- Dataplane: Linux netlink, nftables, eBPF, AF_PACKET, psample, optional VPP " +
		"integrations, and namespace-aware testing.\n")
	out.WriteString("- Release state: pre-release, main-branch builds, no tagged stable release yet.\n")
	out.WriteString("- License and repos: AGPLv3. Canonical repository: " + repositoryURL +
		". Discord: " + discordInvite + ".\n")
	out.WriteString("- Current generated counts: " + strconv.Itoa(facts.Features.CoreExperimental) +
		" shipped or experimental feature cards, " + strconv.Itoa(facts.Features.Planned) +
		" roadmap cards, " + strconv.Itoa(facts.CLICommands) + " CLI commands, " +
		strconv.Itoa(facts.ConfigSections) + " config sections, " + strconv.Itoa(len(inputs.Plugins)) +
		" plugin registrations, " + strconv.Itoa(facts.Dependencies) + " direct Go dependencies, " +
		strconv.Itoa(facts.Changes) + " weekly change entries.\n")
	out.WriteString("- Test evidence counts: " + facts.Tests.UnitDisplay + " unit tests, " +
		facts.Tests.FuzzDisplay + " fuzz targets, " + facts.Tests.E2EDisplay +
		" end-to-end transcript steps, " + strconv.Itoa(facts.Interop.Scenarios) +
		" interop scenarios across " + facts.Interop.TargetDisplay + " target implementations.\n")
	out.WriteString("- Generated date: " + facts.GeneratedAt + ".\n\n")
}

// writeLLMSQualityModel states how Ze proves what it claims, and with which
// commands a reader can re-run each layer.
//
// Every command here is an action this repository registers today. The retired
// renderer named eight `make` targets, and `make` went with the interpreter
// cutover, so a reader who copied that paragraph got "no rule to make target".
func writeLLMSQualityModel(out *strings.Builder, inputs *llmsInputs) {
	tests := inputs.Facts.Tests
	out.WriteString("## Quality and verification model\n\n")
	out.WriteString("Ze uses layered proof because bugs appear at different boundaries.\n\n")
	out.WriteString("- Local Go tests: package behavior, parser rules, encoders, state transitions, " +
		"validation paths, and error shapes. Current scale: " + tests.UnitDisplay + " unit tests.\n")
	out.WriteString("- Race, coverage, and fuzz: fuzz targets are normal Go tests with generated " +
		"input. Current scale: " + tests.FuzzDisplay + " fuzz targets.\n")
	out.WriteString("- gomu mutation checks: mutate production Go code and rerun tests to find weak " +
		"assertions. gomu is advisory, not the default CI gate.\n")
	out.WriteString("- Functional `.ci` transcripts: drive processes, CLI commands, files, HTTP, " +
		"syslog, peers, daemons, exits, and BGP wire expectations. BGP failures are decoded " +
		"structurally, not shown as raw hex only.\n")
	out.WriteString("- Browser `.wb` transcripts: drive the rendered web UI through real browser flows.\n")
	out.WriteString("- Editor `.et` transcripts: drive the headless interactive editor.\n")
	out.WriteString("- QEMU: runs Linux-only behavior from macOS or CI where netlink, nftables, eBPF, " +
		"PPP, network namespaces, and kernel modules exist.\n")
	out.WriteString("- Interop: " + strconv.Itoa(inputs.Facts.Interop.Scenarios) + " scenarios against " +
		inputs.Facts.Interop.TargetDisplay + " target implementations, including FRR, BIRD, GoBGP, " +
		"RustyBGP, OpenBGPD, ExaBGP, and other real daemons where applicable.\n")
	out.WriteString("- Verify workflow: `./le verify worktree` runs the whole native verification " +
		"population against a fixed commit in a detached worktree, writes stage logs under `tmp/`, " +
		"groups related failures, and prints narrow rerun commands. `./le repository` is the " +
		"narrower handoff gate.\n")
	out.WriteString("- Rule for regressions: do not hide a failure with a skip or loose assertion. " +
		"Move the proof to the layer that can see the real behavior, add the narrow test, rerun it, " +
		"then rerun the gate that should have caught it.\n\n")
	out.WriteString("Useful commands: `go test -race -run TestName ./internal/...`, `./le fuzz run`, " +
		"`gomu run`, `bin/ze-test bgp plugin 42 -v`, `./le qemu netns-test`, " +
		"`./le integration interop`, `./le evidence release-candidate`.\n\n")
}

// writeLLMSComparison states which daemons Ze is compared with, and on what
// evidence a comparison claim rests.
func writeLLMSComparison(out *strings.Builder) {
	out.WriteString("## Comparison positioning\n\n")
	out.WriteString("- BGP comparison lens: Ze is compared with BIRD, FRR, OpenBGPD, GoBGP, bio-rd, " +
		"ExaBGP, RustyBGP, rustbgpd, and freeRtr across AFI/SAFI, core protocol, policy, security, " +
		"observability, APIs, operations, and best-path behavior.\n")
	out.WriteString("- Network OS lens: Ze is compared with VyOS and freeRtr across routing, " +
		"interfaces, firewall, NAT, VPN, AAA, services, management APIs, automation, packaging, " +
		"observability, tests, and implementation model.\n")
	out.WriteString("- Evidence policy: capability claims should cite upstream code, official feature " +
		"documentation, or the integration layer that owns the behavior. `Unclear`, `Partial`, and " +
		"`Not found` are valid outcomes when evidence does not support a stronger claim.\n")
	out.WriteString("- Comparison pages are advice for product decisions, not marketing copy.\n\n")
}

// writeLLMSFeatures lists every feature card in the order website/data states.
func writeLLMSFeatures(out *strings.Builder, inputs *llmsInputs) {
	out.WriteString("## Feature inventory\n\n")
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
		out.WriteString("### " + cleanInline(section.Heading) + " (" + strconv.Itoa(len(section.Cards)) +
			" cards: " + strings.Join(summary, ", ") + ")\n")
		if lead := cleanInline(section.Lead); lead != "" {
			out.WriteString(lead + "\n")
		}
		for cardIndex := range section.Cards {
			out.WriteString(featureCardLine(&section.Cards[cardIndex]) + "\n")
		}
		out.WriteString("\n")
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
func writeLLMSConfigRoots(out *strings.Builder, inputs *llmsInputs) {
	out.WriteString("## Configuration model roots\n\n")
	out.WriteString("Top-level YANG-derived config roots. Child names are direct children only, " +
		"enough to orient without fetching the full reference.\n\n")
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
		out.WriteString("- `" + name + "`: " + description + " Children: " + listed + ".\n")
	}
	out.WriteString("\n")
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
func writeLLMSPlugins(out *strings.Builder, inputs *llmsInputs) {
	out.WriteString("## Plugin registry\n\n")
	out.WriteString("Each registration comes from the Go runtime registry. Config roots come from " +
		"plugin metadata and YANG files.\n\n")
	plugins := make([]registryPlugin, len(inputs.Plugins))
	copy(plugins, inputs.Plugins)
	sort.Slice(plugins, func(left, right int) bool { return plugins[left].Name < plugins[right].Name })
	for index := range plugins {
		plugin := &plugins[index]
		out.WriteString("- `" + cleanInline(plugin.Name) + "`: " + trimInline(plugin.Description, 170) +
			" Config roots: " + joinOrNone(plugin.ConfigRoots) +
			". Dependencies: " + joinOrNone(plugin.Dependencies) +
			". Optional: " + joinOrNone(plugin.OptionalDependencies) +
			". YANG files: " + strconv.Itoa(len(plugin.YangFiles)) +
			". Source: `" + cleanInline(plugin.SourceDir) + "`.\n")
	}
	out.WriteString("\n")
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
func writeLLMSCommands(out *strings.Builder, inputs *llmsInputs) {
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

	out.WriteString("## CLI command surface\n\n")
	out.WriteString("The command catalog is generated from `ze help command --json`, not " +
		"hand-written. Modes: " + strings.Join(counted, ", ") + ".\n")
	out.WriteString("`daemon` commands require a running Ze daemon. `read-only` commands query " +
		"state. `offline` commands can run without daemon state. `pipes` groups operators as " +
		"`always`, `with-rows`, `when-streaming`, or `local-only`; these qualifiers are part of " +
		"the contract. An operator absent from those groups is refused by name.\n\n")
	for _, verb := range verbs {
		group := byVerb[verb]
		out.WriteString("### `" + verb + "` commands (" + strconv.Itoa(len(group)) + ")\n")
		for _, command := range group {
			// The whole summary, with no character budget: it is declared as
			// one line, so there is nothing left for a cut to do but stop a
			// sentence mid-clause.
			out.WriteString("- `" + command.Path + "` (" + commandMetadataLine(command) + "): " +
				cleanInline(command.Description) + "\n")
		}
		out.WriteString("\n")
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
func writeLLMSEquivalents(out *strings.Builder, inputs *llmsInputs) {
	mapping := inputs.Equivalents
	vendors := mapping.vendorIDs()
	out.WriteString("## Vendor command equivalents\n\n")
	out.WriteString(cleanInline(mapping.Summary) + "\n")
	out.WriteString("Updated: " + cleanInline(mapping.Updated) + ".\n")
	rooted := make([]string, 0, len(vendors))
	for _, vendor := range vendors {
		rooted = append(rooted, mapping.vendorLabel(vendor)+" ("+mapping.Vendors[vendor].RootingModel+")")
	}
	out.WriteString("Vendors: " + strings.Join(rooted, ", ") + ".\n\n")
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
		out.WriteString(strings.Join(parts, ". ") + ".\n")
	}
	out.WriteString("\n")
}

// writeLLMSDependencies states why Ze takes each direct Go module.
func writeLLMSDependencies(out *strings.Builder, inputs *llmsInputs) {
	out.WriteString("## Dependency rationale\n\n")
	out.WriteString("Direct Go modules are grouped by why Ze needs them. This is generated from " +
		"go.mod plus curated rationale, not copied from package names alone.\n\n")
	for index := range inputs.Dependencies.Categories {
		category := &inputs.Dependencies.Categories[index]
		out.WriteString("### " + cleanInline(category.Name) + " (" +
			strconv.Itoa(len(category.Modules)) + ")\n")
		for _, module := range category.Modules {
			out.WriteString("- `" + cleanInline(module.Module) + "`: " + trimInline(module.Why, 240) + "\n")
		}
		out.WriteString("\n")
	}
}

// writeLLMSPageMap lists the curated navigation, dropdown by dropdown.
func writeLLMSPageMap(out *strings.Builder, inputs *llmsInputs) {
	out.WriteString("## Page map\n\n")
	out.WriteString("Every link points to the page Markdown mirror first. The web URL is the " +
		"human-rendered version of the same page.\n\n")
	for index := range inputs.Nav.Dropdowns {
		dropdown := &inputs.Nav.Dropdowns[index]
		out.WriteString("## " + dropdown.Label + "\n\n")
		for _, column := range dropdown.Columns {
			for _, entry := range column {
				if entry.LabelOnly != "" {
					out.WriteString("### " + entry.LabelOnly + "\n")
					continue
				}
				out.WriteString(navEntryLine(entry, inputs))
			}
		}
		out.WriteString("\n")
	}
	out.WriteString("## More\n\n")
	for _, link := range inputs.Nav.TrailingLinks {
		out.WriteString("- [" + link.Label + "](" + mirrorURL(link.Href) + ") (web: " +
			pageURL(link.Href) + ")\n")
	}
	out.WriteString("- [Discord](" + discordInvite + "): community and support\n")
	out.WriteString("- [GitHub](" + repositoryURL + "): canonical repository, issues, wiki\n")
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
