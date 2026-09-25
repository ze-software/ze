// Design: docs/contributing/spec-workflow.md -- roadmap commands and derived index.
package roadmap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/spec/path"
)

// OutputRel is derived output and never an inventory input.
const OutputRel = "plan/roadmap.md"
const area = "spec roadmap"
const valueRef = "ref"

var actions = leaction.New(area,
	leaction.Action{Verb: "list", Why: "report a committed release inventory preview",
		Parameters: []leaction.Parameter{{Keyword: "revision", Value: valueRef, Requirement: leaction.Optional}},
		AnswerArgs: runList},
	leaction.Action{Verb: "update", Why: "regenerate plan/roadmap.md from a committed tree", Writes: true,
		Parameters: []leaction.Parameter{{Keyword: "revision", Value: valueRef, Requirement: leaction.Optional}},
		AnswerArgs: runUpdate},
	leaction.Action{Verb: "compare", Why: "compare endpoint queues without inferring completion",
		Parameters: []leaction.Parameter{
			{Keyword: "from", Value: valueRef, Requirement: leaction.Required},
			{Keyword: "to", Value: valueRef, Requirement: leaction.Required},
		}, AnswerArgs: runCompare},
)

func init() {
	// GroupGenerate rather than GroupReport: plan/roadmap.md is this area's
	// product, and indexCurrent compares it against the tree, which is rung 1
	// of the ladder in leroot/group.go. A report gates nothing AND writes
	// nothing, and `update` writes.
	leroot.Register(area, leroot.GroupGenerate, Answer, registry.Meta{
		ShortHelp: "committed release inventory, repository index, and endpoint changes",
		Mode:      "offline", Section: registry.SectionTest, SubsFunc: actions.Subs,
	})
	leroot.RegisterActions(area, actions.Actions)
	leroot.RegisterShape(area, command.ShapeMap)
	derived.Register(derived.Artifact{
		Path:  OutputRel,
		Feeds: func(_, rel string) bool { return specpath.IsSpec(rel) },
		Rebuild: func(root string) error {
			_, err := Update(context.Background(), root, "")
			return err
		},
		Complete:     indexCurrent,
		SessionStart: derived.SessionStartDefer,
	})
}

// Answer dispatches the grammar declared in the action table.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runList(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	snapshot, err := Collect(context.Background(), root, args.One("revision"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return &snapshot, 0
}

func runUpdate(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Update(context.Background(), root, args.One("revision"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

func runCompare(args leaction.Arguments) (any, int) {
	for _, keyword := range []string{"from", "to"} {
		if args.One(keyword) == "" {
			leaction.ReportError(fmt.Errorf("spec roadmap compare requires %s <ref>", keyword))
			return nil, 2
		}
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Compare(context.Background(), root, args.One("from"), args.One("to"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}

// UpdateReport names exactly which revision the generated path represents.
type UpdateReport struct {
	Path          string `json:"path"`
	Revision      string `json:"revision"`
	InputDigest   string `json:"input-digest"`
	Historical    bool   `json:"historical"`
	Qualification string `json:"qualification"`
}

// Update atomically writes a repository index from the selected committed tree.
func Update(ctx context.Context, root, revision string) (UpdateReport, error) {
	snapshot, err := Collect(ctx, root, revision)
	if err != nil {
		return UpdateReport{}, err
	}
	head, err := resolve(ctx, root, "HEAD")
	if err != nil {
		return UpdateReport{}, err
	}
	historical := snapshot.Revision != head
	content := Markdown(&snapshot, true)
	if historical {
		content = "Historical snapshot: the selected revision differs from current HEAD. Relative links refer to the local checkout, whose files can differ.\n\n" + content
	}
	if err := os.MkdirAll(filepath.Join(root, specpath.Root), 0o750); err != nil {
		return UpdateReport{}, fmt.Errorf("create roadmap output directory: %w", err)
	}
	if err := derived.WriteAtomic(filepath.Join(root, OutputRel), []byte(content)); err != nil {
		return UpdateReport{}, err
	}
	return UpdateReport{Path: OutputRel, Revision: snapshot.Revision, InputDigest: snapshot.InputDigest,
		Historical: historical, Qualification: Qualification}, nil
}

func indexCurrent(root string) bool {
	head, err := resolve(context.Background(), root, "HEAD")
	if err != nil {
		return false
	}
	content, err := os.ReadFile(filepath.Join(root, OutputRel)) //nolint:gosec // Reads the fixed generated index under the operator-selected repository root.
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "<!-- roadmap-revision: "+head+" -->")
}
