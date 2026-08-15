// Design: docs/architecture/web-interface.md -- Template rendering
// Related: handler_config.go -- ConfigViewData, which most of these render with
// Related: config_leaf_input.templ -- the editor the classes below dress

package web

// The config view's own view models and the text its components compute. A
// class list, a URL and a label are values, so Go builds them and the markup
// holds no tag literal (AC-7 of plan/spec-web-templ-migration.md).

// configFlexData is what configFlex renders. A flex node is a flag, a single
// value, or a block, and which one is decided by whether Value and Children are
// set.
//
// NOTHING BUILDS ONE YET. configViewComponent (handler_config_leaf.go) resolves
// config.NodeFlex to no component, because the config view handler carries a
// ConfigViewData and that type holds none of these fields. Recorded in
// plan/journal/silent-fall-through.md.
type configFlexData struct {
	Name       string
	Value      string
	LeafField  LeafField
	LeafFields []LeafField
	Children   []ChildEntry
}

// configBreadcrumbData is what configBreadcrumb renders.
type configBreadcrumbData struct {
	BackURL  string
	Segments []BreadcrumbSegment
}

// configDiffLine is one line of a config diff, classified so the review can
// color it.
type configDiffLine struct {
	Text     string
	IsAdd    bool
	IsDel    bool
	IsChange bool
}

// configCommitData is what configCommit renders: the pending diff, or the
// conflict that stopped the commit.
type configCommitData struct {
	Error         string
	ConflictPaths []string
	Diff          string
	DiffLines     []configDiffLine
}

// configNotificationData is what configNotification renders.
type configNotificationData struct {
	ChangeCount int
	Messages    []string
}

// leafFieldClass is one config field's class list. `modified` marks a leaf the
// editor session changed against the committed config.
func leafFieldClass(modified bool) string {
	if modified {
		return "config-field modified"
	}

	return "config-field"
}

// leafInputClass is a config text or number editor's class list. An unset leaf
// shows the schema default, and `default-value` is what greys it.
func leafInputClass(configured bool) string {
	if configured {
		return "config-input"
	}

	return "config-input default-value"
}

// leafSelectClass is a config enum editor's class list, on the same rule.
func leafSelectClass(configured bool) string {
	if configured {
		return "config-input config-select"
	}

	return "config-input config-select default-value"
}

// configKeyItemClass is one list-key link's class list.
func configKeyItemClass(selected bool) string {
	if selected {
		return "config-key-item selected"
	}

	return "config-key-item"
}

// breadcrumbItemClass is one breadcrumb segment's class list.
func breadcrumbItemClass(active bool) string {
	if active {
		return "breadcrumb-item breadcrumb-active"
	}

	return "breadcrumb-item"
}

// uncommittedChangeText is the label between the pending-change badge and the
// review link. It carries the space on each side, because the badge, the label
// and the link are all inline and both spaces reach the reader.
func uncommittedChangeText(changes int) string {
	if changes == 1 {
		return " uncommitted change "
	}

	return " uncommitted changes "
}

// commandResultClass is a command result card's class list.
func commandResultClass(failed bool) string {
	if failed {
		return "command-result command-error"
	}

	return "command-result"
}

// dashText is a dashboard value that reads as a dash when it is unknown.
func dashText(value string) string {
	if value == "" {
		return "-"
	}

	return value
}

// namedRouter reports whether the router identity is worth showing. "ze" is the
// fallback identity, and a bar that repeats it tells the operator nothing.
func namedRouter(identity string) bool {
	return identity != "" && identity != defaultRouterIdentity
}

// defaultRouterIdentity is the name the resolver falls back to when neither
// system/host nor bgp/router-id is set.
const defaultRouterIdentity = "ze"
