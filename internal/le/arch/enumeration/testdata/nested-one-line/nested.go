// Two literals that open and close on ONE line, around one copy of the plugin
// names. Shaped after internal/component/authz/authz.go:265, where
// `Section{Entries: []Entry{...}}` reported the same copy twice while the
// enclosing-unit filter compared line ranges.
package fixture

var profile = Section{Default: allow, Members: []string{"copp", "cos"}}
