// Design: docs/architecture/irr-owner.md -- the AS-SET names the commands complete
// Related: irr_owner_fixture.go -- irrAwaitStored and the IRR test constants this one shares
// Related: fixture.go -- Observe owns startup, assertion reporting and shutdown
package fixture

import (
	"context"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// irrStoredHelp is the help the irr owner's completion prints beside an
// AS-SET whose dataset is stored (internal/plugins/irr/completion.go,
// asSetHelpStored). The fixture cannot import the plugin, so the text is
// copied here, and the unit test TestCommandArgumentOffersStoredASSets holds
// the source.
const irrStoredHelp = "AS-SET with a stored IRR dataset"

// irrCompletedCommands are the irr commands whose argument is an AS-SET.
var irrCompletedCommands = []string{"update irr as-set", "clear irr as-set"}

func init() {
	const name = "plugin/irr-as-set-completion"
	Register(name, func(ctx context.Context, _ []string) error {
		return Observe(ctx, name, &sdk.Registration{}, irrASSetCompletion)
	}, StopsDaemon)
}

// irrCompletion is one candidate of a `system command complete args` answer.
type irrCompletion struct {
	Value     string `json:"value"`
	ShortHelp string `json:"short-help"`
}

// irrASSetCompletion observes test/plugin/irr-as-set-completion.ci. Its goal
// is that a real completion request, the one `ze cli` sends at each Tab
// (internal/component/cli/client/inject.go, wireDaemonArgCompletion), answers
// the AS-SET the forked irr owner stored. It waits for AS-TEST to be stored,
// then asks the daemon to complete each AS-SET command: with no prefix and
// with the prefix "AS-T" the answer holds AS-TEST marked stored, and with a
// prefix no name starts with it holds nothing.
func irrASSetCompletion(ctx context.Context, p *sdk.Plugin) error {
	if _, err := irrAwaitStored(ctx, p, irrTestASSet); err != nil {
		return err
	}
	for _, command := range irrCompletedCommands {
		for _, partial := range []string{"", "AS-T"} {
			candidates, err := irrCompleteArgs(ctx, p, command, partial)
			if err != nil {
				return err
			}
			index := slices.IndexFunc(candidates, func(c irrCompletion) bool { return c.Value == irrTestASSet })
			if index < 0 {
				return fmt.Errorf("%s %q completes %v, which lacks the stored %s", command, partial, candidates, irrTestASSet)
			}
			if candidates[index].ShortHelp != irrStoredHelp {
				return fmt.Errorf("%s offers %s as %q, want %q", command, irrTestASSet, candidates[index].ShortHelp, irrStoredHelp)
			}
		}
		candidates, err := irrCompleteArgs(ctx, p, command, "AS-NOTHING-STARTS-SO")
		if err != nil {
			return err
		}
		if len(candidates) != 0 {
			return fmt.Errorf("%s completes %v for a prefix no stored AS-SET has", command, candidates)
		}
	}
	var tb textbuf.Buffer
	_ = tb.Str("OK: update irr as-set and clear irr as-set complete the stored ").Str(irrTestASSet).Byte('\n').StdErr() //nolint:errcheck // stderr is the report channel; the verdict is already decided
	return nil
}

// irrCompleteArgs asks the daemon for the candidates of command's argument
// that start with partial, as `ze cli` asks at a Tab.
func irrCompleteArgs(ctx context.Context, p *sdk.Plugin, command, partial string) ([]irrCompletion, error) {
	line := `system command complete args "` + command + `"`
	if partial != "" {
		line += ` "` + partial + `"`
	}
	var answer struct {
		Completions []irrCompletion `json:"completions"`
	}
	status, err := Dispatch(ctx, p, line, &answer)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", line, err)
	}
	if status != statusDone {
		return nil, fmt.Errorf("%s: status %s", line, status)
	}
	return answer.Completions, nil
}
