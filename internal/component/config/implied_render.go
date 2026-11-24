// Design: docs/architecture/api/architecture.md -- round-trippable text, set-command, and plain-display notice renderers for ImpliedReport.
// Related: implied.go -- ImpliedReport data types and BuildImpliedReport.
// Related: serialize_set.go -- DetectFormat format detection rules (# comment vs #meta).

package config

import (
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	impliedByPrefix    = "# implied by: "
	impliedPrefix      = "# implied: "
	impliedBlockOpen   = " {\n"
	impliedBracketOpen = " [ "
	busKeywordMessage  = "message "
	impliedIndent1     = "    "
	impliedIndent2     = "        "
	impliedIndent3     = "            "
	impliedIndent4     = "                "
	impliedIndent5     = "                    "
)

// RenderImpliedText renders writtenText together with every implied endpoint,
// participant bus block, and inserted filter annotation from report.
//
// Whole implied blocks are preceded by single-hash `# implied by: <reason>`
// comments and written in valid short-path hierarchical `bus { ... }` syntax so
// they round-trip through DetectFormat (never FormatSetMeta) and Parser.Parse.
// Inline annotations use `# implied: <what and why>` and never emit `##` at
// column 0.
func RenderImpliedText(writtenText string, report ImpliedReport) string {
	if !hasImpliedRenderContent(&report) {
		return writtenText
	}

	annotatedWritten, remainingFilters := annotateWrittenFilterLines(writtenText, report.Filters)

	var out textbuf.Buffer
	if annotatedWritten != "" {
		out.Str(annotatedWritten)
		if !strings.HasSuffix(annotatedWritten, "\n") {
			out.Byte('\n')
		}
	}

	needGap := out.Len() > 0
	writeBlockGap := func() {
		if needGap {
			out.Byte('\n')
		}
		needGap = true
	}

	for i := range report.Endpoints {
		writeBlockGap()
		writeImpliedEndpointText(&out, &report.Endpoints[i])
	}

	for i := range report.Participants {
		p := &report.Participants[i]
		if !participantHasImpliedDisplay(p) {
			continue
		}
		writeBlockGap()
		writeImpliedParticipantText(&out, p)
	}

	if len(remainingFilters) > 0 {
		writeBlockGap()
		writeImpliedFilterComments(&out, remainingFilters)
	}

	return out.String()
}

// RenderImpliedSet renders writtenSet together with implied endpoints,
// participant bus rules, and inserted filter annotations in flat set-command
// format. Every comment starts with `# ` so DetectFormat returns FormatSet
// (never FormatSetMeta) and SetParser parses the statements cleanly.
func RenderImpliedSet(writtenSet string, report ImpliedReport) string {
	if !hasImpliedRenderContent(&report) {
		return writtenSet
	}

	var out textbuf.Buffer
	if writtenSet != "" {
		out.Str(writtenSet)
		if !strings.HasSuffix(writtenSet, "\n") {
			out.Byte('\n')
		}
	}

	for i := range report.Endpoints {
		writeImpliedEndpointSet(&out, &report.Endpoints[i])
	}

	for i := range report.Participants {
		p := &report.Participants[i]
		if !participantHasImpliedDisplay(p) {
			continue
		}
		writeImpliedParticipantSet(&out, p)
	}

	if len(report.Filters) > 0 {
		writeImpliedFilterComments(&out, report.Filters)
	}

	return out.String()
}

// ImpliedNoticeLine returns the single-hash footer comment for `| text` plain
// displays when implied items are active, or "" when report is empty.
func ImpliedNoticeLine(report ImpliedReport) string {
	n := report.Count()
	if n == 0 {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str("# note: ").Int(int64(n)).Str(" implied item(s) active; run with | implied to display them").String()
}

// ImpliedPlainNotice returns the human-readable notice for `ze config show`
// stderr and the editor status bar when implied items are active, or "" when
// report is empty.
func ImpliedPlainNotice(report ImpliedReport) string {
	n := report.Count()
	if n == 0 {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Int(int64(n)).Str(" implied item(s) active — use | implied to view").String()
}

func hasImpliedRenderContent(report *ImpliedReport) bool {
	if len(report.Endpoints) > 0 {
		return true
	}
	if len(report.Filters) > 0 {
		return true
	}
	for i := range report.Participants {
		if participantHasImpliedDisplay(&report.Participants[i]) {
			return true
		}
	}
	return false
}

func participantHasImpliedDisplay(p *ImpliedParticipant) bool {
	if allowIsImplied(p.ReceiveAllowOrigin, p.ReceiveAllowNone, p.ReceiveAllow) {
		return true
	}
	if allowIsImplied(p.SendAllowOrigin, p.SendAllowNone, p.SendAllow) {
		return true
	}
	for i := range p.Receive {
		if ruleNeedsImpliedDisplay(&p.Receive[i]) {
			return true
		}
	}
	for i := range p.Send {
		if ruleNeedsImpliedDisplay(&p.Send[i]) {
			return true
		}
	}
	return false
}

func allowIsImplied(origin string, allowNone bool, allow []string) bool {
	if origin == "" {
		return false
	}
	if origin == ImpliedOriginWritten {
		return false
	}
	if allowNone {
		return true
	}
	return len(allow) > 0
}

func ruleNeedsImpliedDisplay(rule *ImpliedDirectionRule) bool {
	if rule.Origin != ImpliedOriginWritten {
		return true
	}
	if len(rule.ReplacedDefaults) > 0 {
		return true
	}
	return rule.Dormant
}

func writeImpliedEndpointText(out *textbuf.Buffer, ep *ImpliedEndpoint) {
	reason := endpointImpliedReason(ep)
	compiled := ep.CompiledPlugin
	if compiled == "" {
		compiled = ep.Name
	}

	out.Str(impliedByPrefix).Str(sanitizeImpliedComment(reason)).Byte('\n')
	out.Str("bus").Str(impliedBlockOpen)
	out.Str(impliedIndent1).Str("endpoint ").Str(QuoteIfNeeded(ep.Name)).Str(impliedBlockOpen)
	out.Str(impliedIndent2).Str("internal ").Str(QuoteIfNeeded(compiled)).Str(";\n")
	out.Str(impliedIndent1).Str("}\n")
	out.Str("}\n")
}

func writeImpliedEndpointSet(out *textbuf.Buffer, ep *ImpliedEndpoint) {
	reason := endpointImpliedReason(ep)
	compiled := ep.CompiledPlugin
	if compiled == "" {
		compiled = ep.Name
	}

	out.Str(impliedByPrefix).Str(sanitizeImpliedComment(reason)).Byte('\n')
	out.Str("set bus endpoint ").Str(QuoteIfNeeded(ep.Name)).Str(" internal ").Str(QuoteIfNeeded(compiled)).Byte('\n')
}

func endpointImpliedReason(ep *ImpliedEndpoint) string {
	if ep.ImpliedBy != "" {
		return ep.ImpliedBy
	}
	var tb textbuf.Buffer
	if ep.ReasonKind != "" {
		tb.Str(ep.ReasonKind)
		if ep.ReasonSource != "" {
			tb.Byte(' ').Str(ep.ReasonSource)
		}
		return tb.String()
	}
	return "auto-load of " + ep.Name
}

func writeImpliedParticipantText(out *textbuf.Buffer, p *ImpliedParticipant) {
	pPath := participantRenderPath(p)
	if pPath == "" {
		return
	}

	header := participantImpliedHeader(p)
	out.Str(impliedByPrefix).Str(sanitizeImpliedComment(header)).Byte('\n')
	out.Str("bus").Str(impliedBlockOpen)
	out.Str(impliedIndent1).Str("participant ").Str(QuoteIfNeeded(pPath)).Str(impliedBlockOpen)

	writeImpliedDirectionText(
		out,
		busDirectionReceive,
		busTargetKeyFrom,
		p.ReceiveAllow,
		p.ReceiveAllowNone,
		p.ReceiveAllowOrigin,
		p.ReceiveAllowSource,
		p.Receive,
	)
	writeImpliedDirectionText(
		out,
		busDirectionSend,
		busTargetKeyTo,
		p.SendAllow,
		p.SendAllowNone,
		p.SendAllowOrigin,
		p.SendAllowSource,
		p.Send,
	)

	out.Str(impliedIndent1).Str("}\n")
	out.Str("}\n")
}

func writeImpliedDirectionText(
	out *textbuf.Buffer,
	dirName, targetKey string,
	allow []string,
	allowNone bool,
	allowOrigin, allowSource string,
	rules []ImpliedDirectionRule,
) {
	showAllow := allowIsImplied(allowOrigin, allowNone, allow)
	var shownRules []*ImpliedDirectionRule
	for i := range rules {
		if ruleNeedsImpliedDisplay(&rules[i]) {
			shownRules = append(shownRules, &rules[i])
		}
	}
	if !showAllow && len(shownRules) == 0 {
		return
	}

	out.Str(impliedIndent2).Str(dirName).Str(impliedBlockOpen)

	if showAllow {
		out.Str(impliedIndent3).Str("allow").Str(impliedBracketOpen)
		if allowNone {
			out.Str(busLabelNone)
		} else {
			writeQuotedListItems(out, allow)
		}
		out.Str(" ]; ").Str(impliedPrefix).Str(sanitizeImpliedComment(formatOriginSource(allowOrigin, allowSource))).Byte('\n')
	}

	for _, rule := range shownRules {
		annotation := formatRuleAnnotation(rule)
		if !rule.Allowed {
			out.Str(impliedIndent3).Str(impliedPrefix).Str(busKeywordMessage).Str(QuoteIfNeeded(rule.Message)).
				Str(" disallowed (").Str(sanitizeImpliedComment(annotation)).Str(")\n")
			continue
		}

		out.Str(impliedIndent3).Str(busKeywordMessage).Str(QuoteIfNeeded(rule.Message)).Str(impliedBlockOpen)
		if len(rule.Targets) > 0 {
			out.Str(impliedIndent4).Str(targetKey).Str(impliedBracketOpen)
			writeQuotedListItems(out, rule.Targets)
			out.Str(" ];")
			if annotation != "" {
				out.Byte(' ').Str(impliedPrefix).Str(sanitizeImpliedComment(annotation))
			}
			out.Byte('\n')
		} else if annotation != "" {
			out.Str(impliedIndent4).Str(impliedPrefix).Str(sanitizeImpliedComment(annotation)).Byte('\n')
		}

		for _, tagName := range slices.Sorted(maps.Keys(rule.Tags)) {
			vals := rule.Tags[tagName]
			if len(vals) == 0 {
				continue
			}
			out.Str(impliedIndent4).Str("tag ").Str(QuoteIfNeeded(tagName)).Str(impliedBlockOpen)
			out.Str(impliedIndent5).Str("value").Str(impliedBracketOpen)
			writeQuotedListItems(out, vals)
			out.Str(" ];\n")
			out.Str(impliedIndent4).Str("}\n")
		}

		out.Str(impliedIndent3).Str("}\n")
	}

	out.Str(impliedIndent2).Str("}\n")
}

func writeImpliedParticipantSet(out *textbuf.Buffer, p *ImpliedParticipant) {
	pPath := participantRenderPath(p)
	if pPath == "" {
		return
	}

	header := participantImpliedHeader(p)
	out.Str(impliedByPrefix).Str(sanitizeImpliedComment(header)).Byte('\n')

	writeImpliedDirectionSet(
		out,
		pPath,
		busDirectionReceive,
		busTargetKeyFrom,
		p.ReceiveAllow,
		p.ReceiveAllowNone,
		p.ReceiveAllowOrigin,
		p.ReceiveAllowSource,
		p.Receive,
	)
	writeImpliedDirectionSet(
		out,
		pPath,
		busDirectionSend,
		busTargetKeyTo,
		p.SendAllow,
		p.SendAllowNone,
		p.SendAllowOrigin,
		p.SendAllowSource,
		p.Send,
	)
}

func writeImpliedDirectionSet(
	out *textbuf.Buffer,
	pPath, dirName, targetKey string,
	allow []string,
	allowNone bool,
	allowOrigin, allowSource string,
	rules []ImpliedDirectionRule,
) {
	prefix := "set bus participant " + QuoteIfNeeded(pPath) + " " + dirName + " "

	if allowIsImplied(allowOrigin, allowNone, allow) {
		out.Str(impliedPrefix).Str(sanitizeImpliedComment(formatOriginSource(allowOrigin, allowSource))).Byte('\n')
		out.Str(prefix).Str("allow")
		if allowNone {
			out.Byte(' ').Str(busLabelNone)
		} else {
			out.Str(impliedBracketOpen)
			writeQuotedListItems(out, allow)
			out.Str(" ]")
		}
		out.Byte('\n')
	}

	for i := range rules {
		rule := &rules[i]
		if !ruleNeedsImpliedDisplay(rule) {
			continue
		}
		annotation := formatRuleAnnotation(rule)
		if !rule.Allowed {
			out.Str(impliedPrefix).Str(dirName).Byte(' ').Str(busKeywordMessage).Str(QuoteIfNeeded(rule.Message)).
				Str(" disallowed (").Str(sanitizeImpliedComment(annotation)).Str(")\n")
			continue
		}

		if annotation != "" {
			out.Str(impliedPrefix).Str(sanitizeImpliedComment(annotation)).Byte('\n')
		}

		msgPrefix := prefix + busKeywordMessage + QuoteIfNeeded(rule.Message)
		emitted := false
		if len(rule.Targets) > 0 {
			out.Str(msgPrefix).Byte(' ').Str(targetKey).Str(impliedBracketOpen)
			writeQuotedListItems(out, rule.Targets)
			out.Str(" ]\n")
			emitted = true
		}
		for _, tagName := range slices.Sorted(maps.Keys(rule.Tags)) {
			vals := rule.Tags[tagName]
			if len(vals) == 0 {
				continue
			}
			out.Str(msgPrefix).Str(" tag ").Str(QuoteIfNeeded(tagName)).Str(" value").Str(impliedBracketOpen)
			writeQuotedListItems(out, vals)
			out.Str(" ]\n")
			emitted = true
		}
		if !emitted {
			out.Str(msgPrefix).Byte('\n')
		}
	}
}

func participantRenderPath(p *ImpliedParticipant) string {
	if p.Path != "" {
		return p.Path
	}
	if p.EndpointName != "" {
		return plugin.EndpointPath(p.EndpointName)
	}
	return ""
}

func participantImpliedHeader(p *ImpliedParticipant) string {
	reason := firstParticipantReason(p)
	if reason == "" {
		subject := p.EndpointName
		if subject == "" {
			subject = p.Path
		}
		reason = "registration of " + subject
	}
	if p.ParentPath != "" && !strings.Contains(reason, p.ParentPath) {
		var tb textbuf.Buffer
		return tb.Str(reason).Str(" (admitted by ").Str(p.ParentPath).Byte(')').String()
	}
	return reason
}

func firstParticipantReason(p *ImpliedParticipant) string {
	for i := range p.Receive {
		if r := &p.Receive[i]; ruleNeedsImpliedDisplay(r) {
			if s := ruleReasonLabel(r, p); s != "" {
				return s
			}
		}
	}
	for i := range p.Send {
		if r := &p.Send[i]; ruleNeedsImpliedDisplay(r) {
			if s := ruleReasonLabel(r, p); s != "" {
				return s
			}
		}
	}
	if allowIsImplied(p.ReceiveAllowOrigin, p.ReceiveAllowNone, p.ReceiveAllow) {
		if p.ReceiveAllowSource != "" {
			return p.ReceiveAllowSource
		}
		return originReasonLabel(p.ReceiveAllowOrigin, p)
	}
	if allowIsImplied(p.SendAllowOrigin, p.SendAllowNone, p.SendAllow) {
		if p.SendAllowSource != "" {
			return p.SendAllowSource
		}
		return originReasonLabel(p.SendAllowOrigin, p)
	}
	return ""
}

func ruleReasonLabel(rule *ImpliedDirectionRule, p *ImpliedParticipant) string {
	if rule.Source != "" {
		return rule.Source
	}
	return originReasonLabel(rule.Origin, p)
}

func originReasonLabel(origin string, p *ImpliedParticipant) string {
	subject := p.EndpointName
	if subject == "" {
		subject = p.Path
	}
	switch origin {
	case ImpliedOriginRegistered:
		return "registration of " + subject
	case ImpliedOriginInherited:
		if p.ParentPath != "" {
			return "inherited from " + p.ParentPath
		}
		return "inherited"
	case ImpliedOriginRequired:
		return "required for " + subject
	case ImpliedOriginDormant:
		return "dormant on " + subject
	default:
		return origin
	}
}

func formatOriginSource(origin, source string) string {
	if source == "" {
		return origin
	}
	if origin == "" {
		return source
	}
	var tb textbuf.Buffer
	return tb.Str(origin).Str(" (").Str(source).Byte(')').String()
}

func formatRuleAnnotation(rule *ImpliedDirectionRule) string {
	var parts []string
	if rule.Dormant {
		if rule.DormantReason != "" {
			var tb textbuf.Buffer
			parts = append(parts, tb.Str("dormant (").Str(rule.DormantReason).Byte(')').String())
		} else {
			parts = append(parts, ImpliedOriginDormant)
		}
	} else if rule.Origin != "" && rule.Origin != ImpliedOriginWritten {
		parts = append(parts, formatOriginSource(rule.Origin, rule.Source))
	}

	if len(rule.ReplacedDefaults) > 0 {
		var tb textbuf.Buffer
		tb.Str("replaced registered default(s) [ ")
		writeQuotedListItems(&tb, rule.ReplacedDefaults)
		tb.Str(" ]")
		parts = append(parts, tb.String())
	}

	return textbuf.Join(parts, "; ")
}

func writeQuotedListItems(out *textbuf.Buffer, items []string) {
	for i, item := range items {
		if i > 0 {
			out.Byte(' ')
		}
		out.Str(QuoteIfNeeded(item))
	}
}

type filterTargetDirKey struct {
	targetPath string
	direction  string
}

// annotateWrittenFilterLines scans writtenText for existing `filters ...`
// statements inside blocks matching each ImpliedFilterMember's TargetPath and
// Direction, appending inline `# implied: ...` annotations without changing any
// written syntax. Any filter members with no matching written `filters` line
// are returned in remaining for block-level comment output.
func annotateWrittenFilterLines(writtenText string, filters []ImpliedFilterMember) (annotated string, remaining []ImpliedFilterMember) {
	if writtenText == "" {
		return writtenText, filters
	}
	if len(filters) == 0 {
		return writtenText, filters
	}

	byKey := make(map[filterTargetDirKey][]int)
	for i := range filters {
		k := filterTargetDirKey{
			targetPath: filters[i].TargetPath,
			direction:  filters[i].Direction,
		}
		byKey[k] = append(byKey[k], i)
	}

	placed := make([]bool, len(filters))
	lines := strings.Split(writtenText, "\n")

	var pathStack []string
	var frameSizes []int

	for idx, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Strip inline trailing comment outside quotes before inspecting block braces.
		code, hasComment := stripUnquotedComment(trimmed)

		if len(pathStack) >= 2 && isFiltersStatement(code) && !hasComment {
			dir := pathStack[len(pathStack)-1]
			targetPath := "/" + textbuf.Join(pathStack[:len(pathStack)-1], "/")
			k := filterTargetDirKey{targetPath: targetPath, direction: dir}
			if indices := byKey[k]; len(indices) > 0 {
				var note textbuf.Buffer
				note.Byte(' ').Str(impliedPrefix)
				for j, fi := range indices {
					if j > 0 {
						note.Str("; ")
					}
					note.Str(sanitizeImpliedComment(formatImpliedFilterAnnotation(&filters[fi])))
					placed[fi] = true
				}
				lines[idx] = raw + note.String()
			}
		}

		openIdx := strings.IndexByte(code, '{')
		closeIdx := strings.IndexByte(code, '}')
		if openIdx >= 0 && (closeIdx < 0 || openIdx < closeIdx) {
			header := strings.TrimSpace(code[:openIdx])
			header = strings.TrimPrefix(header, "inactive:")
			tokens := strings.Fields(header)
			if len(tokens) > 0 {
				pathStack = append(pathStack, tokens...)
				frameSizes = append(frameSizes, len(tokens))
			} else {
				frameSizes = append(frameSizes, 0)
			}
		}
		if closeIdx >= 0 && len(frameSizes) > 0 {
			popCount := frameSizes[len(frameSizes)-1]
			frameSizes = frameSizes[:len(frameSizes)-1]
			if popCount <= len(pathStack) {
				pathStack = pathStack[:len(pathStack)-popCount]
			}
		}
	}

	for i := range filters {
		if !placed[i] {
			remaining = append(remaining, filters[i])
		}
	}
	return textbuf.Join(lines, "\n"), remaining
}

func isFiltersStatement(code string) bool {
	if strings.HasPrefix(code, "filters ") {
		return true
	}
	if strings.HasPrefix(code, "filters\t") {
		return true
	}
	return strings.HasPrefix(code, "filters[")
}

func stripUnquotedComment(line string) (code string, hasComment bool) {
	inDouble := false
	escaped := false
	for i := range len(line) {
		c := line[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inDouble {
			escaped = true
			continue
		}
		if c == '"' {
			inDouble = !inDouble
			continue
		}
		if c == '#' && !inDouble {
			return strings.TrimSpace(line[:i]), true
		}
	}
	return line, false
}

func writeImpliedFilterComments(out *textbuf.Buffer, filters []ImpliedFilterMember) {
	lastTarget := ""
	for i := range filters {
		m := &filters[i]
		if m.TargetPath != lastTarget {
			if lastTarget != "" {
				out.Byte('\n')
			}
			out.Str(impliedByPrefix).Str(sanitizeImpliedComment(m.TargetPath)).Str(" filter chain\n")
			lastTarget = m.TargetPath
		}
		out.Str(impliedPrefix).Str(sanitizeImpliedComment(formatImpliedFilterAnnotation(m))).Byte('\n')
	}
}

func formatImpliedFilterAnnotation(m *ImpliedFilterMember) string {
	var tb textbuf.Buffer
	if m.Direction != "" {
		tb.Str(m.Direction).Byte(' ')
	}
	if m.Position != "" {
		tb.Str(m.Position).Byte(' ')
	}
	tb.Str(m.Ref)
	if m.ImpliedBy != "" || m.Required {
		tb.Str(" (")
		if m.ImpliedBy != "" {
			tb.Str(m.ImpliedBy)
		}
		if m.Required {
			if m.ImpliedBy != "" {
				tb.Str(", ")
			}
			tb.Str(ImpliedOriginRequired)
			if m.Reason != "" {
				tb.Str(": ").Str(m.Reason)
			}
		}
		tb.Byte(')')
	}
	return tb.String()
}

// sanitizeImpliedComment flattens newlines and leading/trailing spaces so a
// rendered comment can never split onto a new line with a `##` or `#word`
// sequence at column 0.
func sanitizeImpliedComment(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return strings.TrimSpace(s)
	}
	var tb textbuf.Buffer
	for i := range len(s) {
		switch s[i] {
		case '\r', '\n':
			tb.Byte(' ')
		default:
			tb.Byte(s[i])
		}
	}
	return strings.TrimSpace(tb.String())
}
