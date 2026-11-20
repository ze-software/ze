// Design: docs/architecture/api/birdwatcher-compat.md -- the fixture follows the engine
// Related: server_test.go -- mockDispatch answers every `show bgp rib` command through this file

package lg

import (
	"encoding/json"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// The bgp-rib answers this package's tests are fed.
//
// Each answer is the shape the plugin produces, and each one names the producer
// it copies. A fixture written for what the looking glass wished the engine said
// hid two defects at once: the fake answered `show bgp rib best <family>` with
// `{"routes":[...]}`, while the plugin refused that spelling and, given the right
// one, answers under `best-path`. The best-routes table was empty on every router
// and green in every test. `test/plugin/lg-best-table.ci` drives the real plugin
// and is the check that these copies still match it.

// fakeBestPathAnswer is what `show bgp rib best | family ipv4/unicast` answers:
// rows under bestPathEnvelopeKey, each one bestResult
// (internal/component/bgp/plugins/rib/rib_pipeline_best.go), its attributes
// rendered by enrichRouteMapFromEntry.
const fakeBestPathAnswer = `{"best-path":[{"family":"ipv4/unicast","prefix":"10.0.0.0/24",` +
	`"best-peer":"10.0.0.1","attributes":{"next-hop":"10.0.0.1","origin":"igp",` +
	`"as-path":[65001],"local-preference":100}}]}`

// fakeReceivedRow and fakeSentRow are one row each of what `show bgp rib`
// answers with no terminal: rows under showRowsEnvelopeKey, each one
// serializeRouteItem (internal/component/bgp/plugins/rib/rib_pipeline.go),
// naming its own peer and direction as fields. The received row is a route
// 10.0.0.1 sent Ze; the sent row is a route Ze sent 10.0.0.1, under a prefix
// the received side does not hold, so an answer that mixes the two scopes
// shows.
const (
	fakeReceivedRow = `{"peer":"10.0.0.1","direction":"received",` +
		`"family":"ipv4/unicast","prefix":"10.0.0.0/24","next-hop":"10.0.0.1","origin":"igp",` +
		`"as-path":[65001,65002],"local-preference":100,"med":0,` +
		`"community":["65000:100","65001:200"],"large-community":["65000:0:100"]}`
	fakeSentRow = `{"peer":"10.0.0.1","direction":"sent",` +
		`"family":"ipv4/unicast","prefix":"192.0.2.0/24","next-hop":"10.0.0.254","origin":"igp",` +
		`"as-path":[],"local-preference":100}`
)

// fakeRouteRowsAnswer is the `received` scope: the rows 10.0.0.1 sent Ze.
const fakeRouteRowsAnswer = `{"routes":[` + fakeReceivedRow + `]}`

// fakeSentRowsAnswer is the `sent` scope: the rows Ze sent 10.0.0.1.
const fakeSentRowsAnswer = `{"routes":[` + fakeSentRow + `]}`

// fakeBothRowsAnswer is the default scope, `sent-received`, which
// parsePipelineOperations applies when no scope operation is given: both
// directions.
const fakeBothRowsAnswer = `{"routes":[` + fakeReceivedRow + `,` + fakeSentRow + `]}`

// The `count` terminal per scope: showPipeline answers jsonKeyCount, counting
// the rows the scope selected. The default scope counts both directions.
const (
	fakeCountAnswer         = `{"count":100}`
	fakeSentCountAnswer     = `{"count":50}`
	fakeBothCountAnswer     = `{"count":150}`
	fakeHistogramAnswer     = `{"histogram":{"ipv4/unicast":{"24":100}},"count":100}`
	fakeSentHistogramAnswer = `{"histogram":{"ipv4/unicast":{"24":50}},"count":50}`
	fakeBothHistogramAnswer = `{"histogram":{"ipv4/unicast":{"24":150}},"count":150}`
)

// fakeScopeAnswers maps the scope a command selects to its answer for each
// terminal, "" being the route rows. Scope "" is the default, sent-received.
var fakeScopeAnswers = map[string]map[string]string{
	"received":      {"": fakeRouteRowsAnswer, "count": fakeCountAnswer, "histogram": fakeHistogramAnswer},
	"sent":          {"": fakeSentRowsAnswer, "count": fakeSentCountAnswer, "histogram": fakeSentHistogramAnswer},
	"advertised":    {"": fakeSentRowsAnswer, "count": fakeSentCountAnswer, "histogram": fakeSentHistogramAnswer},
	"sent-received": {"": fakeBothRowsAnswer, "count": fakeBothCountAnswer, "histogram": fakeBothHistogramAnswer},
	"":              {"": fakeBothRowsAnswer, "count": fakeBothCountAnswer, "histogram": fakeBothHistogramAnswer},
}

// The pipe operations parsePipelineOperations and parseBestPipelineOperations
// accept (rib_pipeline.go: scopeKeywords, filterKeywords, terminalKeywords,
// and bestTerminalReason in rib_pipeline_best.go). The three source
// selectors, `peer`, `covering` and `exact`, are not in filterKeywords: the
// plugin reads each one by name, and so does fakeRIBPipeline. The `json` word
// is not a RIB terminal: `| json` is the engine's format.
var (
	fakeRIBScopes    = map[string]bool{"advertised": true, "sent": true, "received": true, "sent-received": true}
	fakeRIBFilters   = map[string]bool{"path": true, "aspath": true, "prefix": true, "community": true, "family": true, "match": true, "first": true, "last": true}
	fakeRIBTerminals = map[string]bool{"count": true, "histogram": true, "graph": true}
)

// fakeRIBOperation is one pipe operation of a `show bgp rib` line: its name
// and its argument words, as the engine's partition hands them to the plugin
// (sdk.PipeOperation).
type fakeRIBOperation struct {
	name string
	args []string
}

// The selectors and filters the fake applies rather than only parses, each
// with the meaning the plugin gives it. Every other filter is parsed and
// ignored, which a test cannot mistake for a result, because the fixed rows
// carry only one prefix per direction.
const (
	// fakeRIBPeer selects the peers, and the fake reads only its value.
	fakeRIBPeer = "peer"
	// fakeRIBCovering keeps the rows whose prefix contains one address, the
	// most specific first within each peer.
	fakeRIBCovering = "covering"
	// fakeRIBExact keeps the rows of exactly one prefix.
	fakeRIBExact = "exact"
	// fakeRIBPrefix keeps a row whose prefix text starts with the value
	// (prefixFilter in rib_pipeline.go).
	fakeRIBPrefix = "prefix"
	// fakeRIBPath is validated as validatePathPattern does, and both it and
	// fakeRIBASPath keep the rows matchASPath keeps.
	fakeRIBPath   = "path"
	fakeRIBASPath = "aspath"
)

// fakeRIBQuery is what fakeRIBPipeline read from one command.
type fakeRIBQuery struct {
	scope    string
	terminal string
	covering netip.Addr   // valid only when the command carried `covering`
	exact    netip.Prefix // valid only when the command carried `exact`
	prefixes []string     // the value of every `prefix` text filter
	paths    []string     // the value of every `path` and `aspath` filter
}

// selects reports whether the query keeps only some of the fixed rows.
func (q fakeRIBQuery) selects() bool {
	if q.covering.IsValid() || q.exact.IsValid() {
		return true
	}
	return len(q.prefixes) > 0 || len(q.paths) > 0
}

// fakeRIBAnswer answers one `show bgp rib ...` line as bgp-rib would.
//
// It refuses what the plugin refuses, with the plugin's error envelope: a word
// on the command path after `show bgp rib` or `show bgp rib best` (the
// argument form, refuseRouteReadArguments in rib_commands.go), an operation it
// does not know, and anything after a terminal. A fake that answers every
// spelling cannot catch a caller that sends the wrong one.
func fakeRIBAnswer(cmd string) string {
	query, best, refusal := fakeRIBRead(cmd)
	if refusal != "" {
		return `{"error":` + strconv.Quote(refusal) + `}`
	}
	scope, terminal := query.scope, query.terminal

	if query.selects() {
		if best {
			return fakeSelectedAnswer(fakeBestPathAnswer, "best-path", "", query)
		}
		return fakeSelectedAnswer(fakeScopeAnswers[scope][""], "routes", "peer", query)
	}

	if best {
		switch terminal {
		case "count":
			return fakeCountAnswer
		case "histogram":
			return fakeHistogramAnswer
		}
		return fakeBestPathAnswer
	}
	switch terminal {
	case "count", "histogram":
	default:
		// The fake has no shape of its own for `graph`, so it answers the
		// route rows, as it did before scopes were told apart.
		terminal = ""
	}
	return fakeScopeAnswers[scope][terminal]
}

// fakeRIBRead splits one `show bgp rib` line the way the engine's partition
// and the plugin's command lookup do: the command part before the first `|`
// names `show bgp rib` or `show bgp rib best`, and each `|` after it starts one
// pipe operation, its first word the name and the rest its argument words.
// It answers what fakeRIBPipeline reads from those operations, whether the
// line is the best-path read, or the reason the plugin refuses it. A word on
// the command path past the reader's own is the argument form, refused as
// refuseRouteReadArguments refuses it: the fake carries the problem alone.
func fakeRIBRead(cmd string) (fakeRIBQuery, bool, string) {
	segments := strings.Split(cmd, "|")
	path := strings.Fields(strings.TrimPrefix(strings.TrimSpace(segments[0]), "show bgp rib"))
	best := len(path) > 0 && path[0] == "best"
	reader := "show bgp rib"
	if best {
		path = path[1:]
		reader = "show bgp rib best"
	}
	if len(path) > 0 {
		return fakeRIBQuery{}, best, reader + " takes its selection after a pipe, and was given the argument " + path[0]
	}

	operations := make([]fakeRIBOperation, 0, len(segments)-1)
	for _, segment := range segments[1:] {
		words := strings.Fields(segment)
		if len(words) == 0 {
			return fakeRIBQuery{}, best, "empty pipe operation"
		}
		operations = append(operations, fakeRIBOperation{name: words[0], args: words[1:]})
	}
	query, refusal := fakeRIBPipeline(operations, best)
	return query, best, refusal
}

// fakeRIBValue reads the one value of a value-taking operation as
// operationValue does: `match` joins its words with one space, every other
// operation takes exactly one word.
func fakeRIBValue(operation fakeRIBOperation, missing string) (string, string) {
	if len(operation.args) == 0 {
		return "", missing
	}
	if operation.name == "match" {
		return strings.Join(operation.args, " "), ""
	}
	if len(operation.args) == 1 {
		return operation.args[0], ""
	}
	return "", operation.name + " takes one value: quote a value that holds a space, as in | " +
		operation.name + ` "` + strings.Join(operation.args, " ") + `"`
}

// fakeRIBNoValue refuses an argument word on a scope or a terminal, as
// operationTakesNoValue does.
func fakeRIBNoValue(operation fakeRIBOperation) string {
	if len(operation.args) == 0 {
		return ""
	}
	return operation.name + " takes no value, and was given " + operation.args[0]
}

// fakeRIBPipeline walks the operations the way the plugin's parser does and
// answers the scope, the terminal it ends in and the rows it selects, or the
// reason it refuses them. The best-path walk takes no scope operation and
// accepts the `reason` terminal. The plugin accepts one scope operation
// anywhere before the terminal and refuses a second.
//
// `covering` and `exact` are refused where the plugin refuses them, in the
// plugin's words (rib_pipeline_covering.go): by name on `show bgp rib best`,
// before anything else of the operation is read (coveringOnBestRefusal,
// exactOnBestRefusal in parseBestPipelineOperations); a second one of the same
// operation, a missing value, and a value that is not the address or the
// prefix each takes (parsePipelineOperations, parseCoveringAddress,
// parseExactPrefix); both together, at the second one once its value parsed
// (walkConflictRefusal); and on any scope but `received`, after the parse
// (refuseCoveringScope, refuseExactScope, checked by showPipeline). The fake
// carries the reason alone: refusePipeline wraps it in an operator error
// whose second and third lines are its why and its fix.
func fakeRIBPipeline(operations []fakeRIBOperation, best bool) (fakeRIBQuery, string) {
	var query fakeRIBQuery
	peerSelected := false
	for _, operation := range operations {
		keyword := operation.name
		if !best && fakeRIBScopes[keyword] {
			if refusal := fakeRIBNoValue(operation); refusal != "" {
				return fakeRIBQuery{}, refusal
			}
			if query.terminal != "" {
				return fakeRIBQuery{}, "filter after terminal: " + keyword
			}
			if query.scope != "" {
				return fakeRIBQuery{}, "multiple route direction filters: " + query.scope + " and " + keyword
			}
			query.scope = keyword
			continue
		}
		if fakeRIBTerminals[keyword] || (best && keyword == "reason") {
			if refusal := fakeRIBNoValue(operation); refusal != "" {
				return fakeRIBQuery{}, refusal
			}
			if query.terminal != "" {
				return fakeRIBQuery{}, "multiple terminals not allowed"
			}
			query.terminal = keyword
			continue
		}
		if best {
			if keyword == fakeRIBCovering {
				return fakeRIBQuery{}, "covering is not available on show bgp rib best: " +
					"it reads received routes, so use show bgp rib | received | covering <address>"
			}
			if keyword == fakeRIBExact {
				return fakeRIBQuery{}, "exact is not available on show bgp rib best: " +
					"it reads received routes, so use show bgp rib | received | exact <prefix>"
			}
		}
		known := keyword == fakeRIBPeer || fakeRIBFilters[keyword]
		if !best {
			known = known || keyword == fakeRIBCovering || keyword == fakeRIBExact
		}
		if !known {
			return fakeRIBQuery{}, "unknown keyword: " + keyword
		}
		if query.terminal != "" {
			return fakeRIBQuery{}, "filter after terminal: " + keyword
		}
		refusal := fakeRIBFilter(&query, operation, &peerSelected)
		if refusal != "" {
			return fakeRIBQuery{}, refusal
		}
	}
	if query.covering.IsValid() && query.scope != "received" {
		return fakeRIBQuery{}, "covering reads received routes only, not sent routes"
	}
	if query.exact.IsValid() && query.scope != "received" {
		return fakeRIBQuery{}, "exact reads received routes only, not sent routes"
	}
	return query, ""
}

// fakeRIBFilter reads one value-taking operation into query: a source
// selector (`peer`, `covering`, `exact`) or a filter, refusing a second
// selector of one kind and a value its producer refuses.
func fakeRIBFilter(query *fakeRIBQuery, operation fakeRIBOperation, peerSelected *bool) string {
	keyword := operation.name
	switch keyword {
	case fakeRIBPeer:
		if *peerSelected {
			return "duplicate peer filter"
		}
		if _, refusal := fakeRIBValue(operation, "peer requires a value"); refusal != "" {
			return refusal
		}
		*peerSelected = true
	case fakeRIBCovering:
		if query.covering.IsValid() {
			return "duplicate covering filter: one address per lookup"
		}
		value, refusal := fakeRIBValue(operation, "covering requires a value: an IP address such as 192.0.2.1")
		if refusal != "" {
			return refusal
		}
		addr, refusal := fakeCoveringAddress(value)
		if refusal != "" {
			return refusal
		}
		if query.exact.IsValid() {
			return fakeWalkConflictRefusal(addr, query.exact)
		}
		query.covering = addr
	case fakeRIBExact:
		if query.exact.IsValid() {
			return "duplicate exact filter: one prefix per lookup"
		}
		value, refusal := fakeRIBValue(operation, "exact requires a value: a prefix with its mask such as 10.0.0.0/8")
		if refusal != "" {
			return refusal
		}
		prefix, refusal := fakeExactPrefix(value)
		if refusal != "" {
			return refusal
		}
		if query.covering.IsValid() {
			return fakeWalkConflictRefusal(query.covering, prefix)
		}
		query.exact = prefix
	default:
		value, refusal := fakeRIBValue(operation, keyword+" requires a value")
		if refusal != "" {
			return refusal
		}
		switch keyword {
		case fakeRIBPrefix:
			query.prefixes = append(query.prefixes, value)
		case fakeRIBPath:
			if refusal := fakePathPatternRefusal(value); refusal != "" {
				return refusal
			}
			query.paths = append(query.paths, value)
		case fakeRIBASPath:
			query.paths = append(query.paths, value)
		}
	}
	return ""
}

// fakeWalkConflictRefusal words the refusal of a command naming both covering
// and exact as walkConflictRefusal does.
func fakeWalkConflictRefusal(addr netip.Addr, prefix netip.Prefix) string {
	return "covering and exact select different routes, so one lookup takes one of them: " +
		"drop exact to list the routes containing " + addr.String() +
		", or drop covering to list the routes of " + prefix.String()
}

// fakeCoveringAddress reads a covering value as parseCoveringAddress and
// coveringValueRefusal do: one address, no mask, no zone.
func fakeCoveringAddress(value string) (netip.Addr, string) {
	addr, err := netip.ParseAddr(value)
	if err != nil {
		if prefix, prefixErr := netip.ParsePrefix(value); prefixErr == nil {
			return netip.Addr{}, "covering takes an IP address with no mask, such as " + prefix.Addr().String() +
				": " + strconv.Quote(value) + " carries a mask; for the routes of that prefix itself, use exact " +
				prefix.Masked().String()
		}
		example := "192.0.2.1"
		if strings.Contains(value, ":") {
			example = "2001:db8::1"
		}
		return netip.Addr{}, "covering takes an IP address with no mask, such as " + example + ": " +
			strconv.Quote(value) + " is not an IP address"
	}
	if addr.Zone() != "" {
		return netip.Addr{}, "covering takes an IP address with no zone, such as " + addr.WithZone("").String() +
			": " + strconv.Quote(value) + " names a zone"
	}
	return addr, ""
}

// fakeExactPrefix reads an exact value as parseExactPrefix and
// exactValueRefusal do: one prefix with its mask, no host bits set and no
// zone.
func fakeExactPrefix(value string) (netip.Prefix, string) {
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		if strings.Contains(value, "%") {
			return netip.Prefix{}, "exact takes a prefix with no zone, such as " + fakeExactZonelessExample(value) +
				": " + strconv.Quote(value) + " names a zone"
		}
		if addr, addrErr := netip.ParseAddr(value); addrErr == nil {
			return netip.Prefix{}, "exact takes a prefix with its mask, such as " +
				netip.PrefixFrom(addr, addr.BitLen()).String() + ": " + strconv.Quote(value) +
				" is an address with no mask; for the routes containing that address, use covering " + value
		}
		return netip.Prefix{}, "exact takes a prefix with its mask, such as " + fakeExactExample(value) + ": " +
			strconv.Quote(value) + " is not a prefix"
	}
	if prefix.Masked() != prefix {
		return netip.Prefix{}, "exact takes a prefix with no host bits set, such as " + prefix.Masked().String() +
			": " + strconv.Quote(value) + " sets bits past its mask"
	}
	return prefix, ""
}

// fakeExactZonelessExample is the example a zoned exact value is offered, as
// exactZonelessExample answers it: the value with its zone dropped and its
// mask kept, a zoned address's host prefix, or the generic example.
func fakeExactZonelessExample(value string) string {
	address, rest, _ := strings.Cut(value, "%")
	if _, mask, hasMask := strings.Cut(rest, "/"); hasMask {
		if prefix, err := netip.ParsePrefix(address + "/" + mask); err == nil {
			return prefix.Masked().String()
		}
		return fakeExactExample(value)
	}
	if addr, err := netip.ParseAddr(address); err == nil {
		return netip.PrefixFrom(addr, addr.BitLen()).String()
	}
	return fakeExactExample(value)
}

// fakeExactExample is the prefix an exact refusal offers when the value gives
// none, as exactExample answers it: of the IP version the value looks like.
func fakeExactExample(value string) string {
	if strings.Contains(value, ":") {
		return "2001:db8::/32"
	}
	return "10.0.0.0/8"
}

// fakePathPatternBody answers the AS numbers part of a path value, without
// its anchors, and whether a $ closed it. The engine reads the end anchor
// once, directly after the last AS number (parsePathPattern,
// internal/component/bgp/plugins/rib/rib_pipeline.go), so a $ anywhere else,
// or one that follows no digit, is the refusal.
func fakePathPatternBody(pattern string) (body string, endAnchored bool, refusal string) {
	body, endAnchored = strings.CutSuffix(strings.TrimPrefix(pattern, "^"), "$")
	if strings.Contains(body, "$") {
		return "", false, "invalid end anchor in path pattern: " + pattern
	}
	if !endAnchored {
		return body, false, ""
	}
	if body == "" {
		return "", false, "invalid end anchor in path pattern: " + pattern
	}
	if last := body[len(body)-1]; last < '0' || last > '9' {
		return "", false, "invalid end anchor in path pattern: " + pattern
	}
	return body, true, ""
}

// fakePathPatternRefusal checks a path value as validatePathPattern does:
// one optional leading ^, then AS numbers joined by commas, each read by
// asn.Parse, empty pieces skipped, then one optional $ directly after the
// last AS number.
func fakePathPatternRefusal(pattern string) string {
	body, _, refusal := fakePathPatternBody(pattern)
	if refusal != "" {
		return refusal
	}
	for piece := range strings.SplitSeq(body, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		if _, err := asn.Parse(piece); err != nil {
			return "invalid ASN in path pattern: " + piece
		}
	}
	return ""
}

// fakeMatchASPath keeps a row as matchASPath does: the pattern's AS numbers
// as a contiguous run anywhere in the path, at its start after a leading ^,
// at its end before a closing $, or as the whole path between both. A pattern
// holding no AS number keeps every row; one holding an AS number asn.Parse
// refuses, or a misplaced $, keeps none.
func fakeMatchASPath(path []uint32, pattern string) bool {
	anchored := strings.HasPrefix(pattern, "^")
	body, endAnchored, refusal := fakePathPatternBody(pattern)
	if refusal != "" {
		return false
	}
	var needles []uint32
	for piece := range strings.SplitSeq(body, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		number, err := asn.Parse(piece)
		if err != nil {
			return false
		}
		needles = append(needles, number)
	}
	if len(needles) == 0 {
		return true
	}
	if anchored && endAnchored {
		return slices.Equal(path, needles)
	}
	if anchored {
		return len(path) >= len(needles) && slices.Equal(path[:len(needles)], needles)
	}
	if endAnchored {
		return len(path) >= len(needles) && slices.Equal(path[len(path)-len(needles):], needles)
	}
	for start := 0; start+len(needles) <= len(path); start++ {
		if slices.Equal(path[start:start+len(needles)], needles) {
			return true
		}
	}
	return false
}

// fakeRowASPath is the AS path of one fixed row: the row's own `as-path`, or
// the one under `attributes` in a best-path row.
func fakeRowASPath(row map[string]any) []uint32 {
	raw, has := row["as-path"].([]any)
	if !has {
		attributes, _ := row["attributes"].(map[string]any)
		raw, _ = attributes["as-path"].([]any)
	}
	path := make([]uint32, 0, len(raw))
	for _, hop := range raw {
		text, isNumber := hop.(json.Number)
		if !isNumber {
			panic("BUG: fixed rib row carries an AS path hop that is not a number")
		}
		number, err := strconv.ParseUint(string(text), 10, 32)
		if err != nil {
			panic("BUG: fixed rib row carries an AS path hop that is not an AS number")
		}
		path = append(path, uint32(number))
	}
	return path
}

// fakeRowSelected reports whether the query keeps one fixed row.
func fakeRowSelected(row map[string]any, query fakeRIBQuery) bool {
	text, _ := row["prefix"].(string)
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		panic("BUG: fixed rib row carries no prefix: " + text)
	}
	if query.covering.IsValid() && !prefix.Contains(query.covering) {
		return false
	}
	if query.exact.IsValid() && prefix != query.exact {
		return false
	}
	for _, pattern := range query.prefixes {
		if !strings.HasPrefix(text, pattern) {
			return false
		}
	}
	for _, pattern := range query.paths {
		if !fakeMatchASPath(fakeRowASPath(row), pattern) {
			return false
		}
	}
	return true
}

// fakeSelectedAnswer answers a query over the rows of one fixed answer: the
// rows under key the query keeps (fakeRowSelected). A covering lookup lists
// them most specific first within each value of peerField (all rows are one
// group when peerField is ""), the peers in the order they first appear;
// every other query keeps the fixed order within each peer. The `count` and
// `histogram`
// terminals count those rows, in the shapes the fixed answers above use.
func fakeSelectedAnswer(answer, key, peerField string, query fakeRIBQuery) string {
	decoder := json.NewDecoder(strings.NewReader(answer))
	decoder.UseNumber()
	var body map[string][]map[string]any
	if err := decoder.Decode(&body); err != nil {
		panic("BUG: fixed rib answer does not decode: " + err.Error())
	}

	var peers []string
	groups := map[string][]map[string]any{}
	for _, row := range body[key] {
		if !fakeRowSelected(row, query) {
			continue
		}
		peer, _ := row[peerField].(string)
		if _, seen := groups[peer]; !seen {
			peers = append(peers, peer)
		}
		groups[peer] = append(groups[peer], row)
	}

	rows := []map[string]any{}
	for _, peer := range peers {
		group := groups[peer]
		if query.covering.IsValid() {
			slices.SortStableFunc(group, func(a, b map[string]any) int {
				return fakeRowBits(b) - fakeRowBits(a)
			})
		}
		rows = append(rows, group...)
	}

	var out any
	switch query.terminal {
	case "count":
		out = map[string]any{"count": len(rows)}
	case "histogram":
		histogram := map[string]map[string]int{}
		for _, row := range rows {
			family, _ := row["family"].(string)
			if histogram[family] == nil {
				histogram[family] = map[string]int{}
			}
			histogram[family][strconv.Itoa(fakeRowBits(row))]++
		}
		out = map[string]any{"histogram": histogram, "count": len(rows)}
	default:
		out = map[string]any{key: rows}
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		panic("BUG: covering answer does not encode: " + err.Error())
	}
	return string(encoded)
}

// fakeRowBits is the prefix length of one fixed row. fakeCoveringAnswer has
// already parsed every row it passes here.
func fakeRowBits(row map[string]any) int {
	text, _ := row["prefix"].(string)
	return netip.MustParsePrefix(text).Bits()
}
