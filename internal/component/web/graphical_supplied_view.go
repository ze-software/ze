// Design: docs/architecture/web-components.md -- registered caller-input views.
package web

// GraphicalSuppliedPanel is a request-owned projection of registered commands.
// Nil means this handler was not given an operational command tree.
type GraphicalSuppliedPanel struct {
	Choices []GraphicalSuppliedChoice
	Input   *GraphicalSuppliedInputs
	Error   string
}

type GraphicalSuppliedChoice struct {
	Path  string
	Label string
}

type GraphicalSuppliedInputs struct {
	Command     string
	Summary     string
	Description string
	Limit       string
	Sources     []GraphicalSuppliedSource
	Options     []GraphicalSuppliedOption
}

type GraphicalSuppliedSource struct {
	Label     string
	Field     string
	FileField string
	ContentID string
	FileID    string
}

type GraphicalSuppliedOption struct {
	Name        string
	Label       string
	Description string
	Flag        bool
	Required    bool
	Choices     []string
}

type GraphicalSuppliedAnswer struct {
	Output string
	Failed bool
}
