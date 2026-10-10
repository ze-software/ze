package cli

import "github.com/ze-software/ze/internal/component/config"

// setWorkingContent replaces the editor's working content with content and
// parses it into the tree, falling back to raw text (treeValid false) when it
// does not parse. Tests use it to stage an edited config without a command;
// the editor itself changes content only through its edit primitives.
func (e *Editor) setWorkingContent(content string) {
	e.workingContent = content
	if e.schema != nil {
		parser := config.NewParser(e.schema)
		tree, err := parser.Parse(content)
		if err == nil {
			e.tree = tree
			e.treeValid = true
		} else {
			e.treeValid = false
		}
	}
}
