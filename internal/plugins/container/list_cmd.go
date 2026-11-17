// Design: docs/architecture/container/runtime.md -- the container listing: its path, proxy, handler and rows
// Related: container.go -- commandDecls and handleCommand, which declare and answer it
// Related: register.go -- where its proxy is registered
// Related: supervise.go -- the supervisor records the listing reads

package container

import (
	"slices"
	"strings"
	"time"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The container listing: the plugin command it declares, the wire method its
// YANG node binds (yang/ze-container-cmd.yang), and the key its rows are
// under in the answer.
const (
	cmdShowList   = "show container list"
	wireShowList  = "ze-container:show-list"
	keyContainers = "containers"
)

// The reasons of a container that holds a state with no record of its own
// beside it: a start of it is in progress or due, and, as the runtime states
// it, a start in progress shows the state it left.
const (
	reasonNoStartYet      = "no start of it has ended yet"
	reasonStartInProgress = "its image is resolved again: a start of it is in progress"
)

// listCommandDecl is the listing's Stage 1 declaration: rows, in the column
// order of the operator surface's command table. The table also names
// restart-count and last-exit-status: no restart policy and no exit record
// exist yet, so the rows carry neither.
func listCommandDecl() sdk.CommandDecl {
	return sdk.CommandDecl{
		Name:      cmdShowList,
		ShortHelp: "Show every configured container, its state and the image it runs.",
		// The rows are under "containers" (answerContainerList).
		Shape:   shapeTable,
		Columns: []string{keyName, keyState, "reason", "image", "digest", "since"},
	}
}

// listCommandRPC is the proxy the listing's YANG command node binds: it
// forwards the command to this plugin's process, internal or forked.
func listCommandRPC() pluginserver.RPCRegistration {
	return pluginserver.RPCRegistration{WireMethod: wireShowList, Handler: forwardToPlugin(cmdShowList), PluginCommand: cmdShowList}
}

// ContainerRow is one row of show container list: one configured container.
type ContainerRow struct {
	Name  string `json:"name"`
	State string `json:"state"`
	// Reason is one sentence in the operator's terms: why a container waits
	// for its image, why it failed, or that no start of it has ended yet.
	Reason string `json:"reason,omitempty"`
	// Image is the whole reference the container's image leaf completes to.
	Image string `json:"image"`
	// Digest and Since are the image a running container runs and when its
	// entrypoint was released. No other state carries them.
	Digest string    `json:"digest,omitempty"`
	Since  time.Time `json:"since,omitzero"`
}

// answerContainerList answers show container list: one row for each
// configured container, sorted by name.
func answerContainerList(sup *supervisor) (string, any, error) {
	rows, err := sup.containerRows()
	if err != nil {
		return rpc.StatusError, nil, err
	}
	return rpc.StatusDone, map[string]any{keyContainers: rows}, nil
}

// containerRows answers one row for each configured container, sorted by
// name. Every row is read under one hold of the supervisor's lock, so a state
// and the record that details it come from the same moment. It never takes a
// container's lifecycle lock: a start or a stop in progress, which holds that
// lock for as long as the start takes, never delays the listing.
func (s *supervisor) containerRows() ([]ContainerRow, error) {
	s.mu.Lock()
	rows := make([]ContainerRow, 0, len(s.desired))
	// images holds the configured image leaf of each row, at the same index,
	// completed once the lock is released.
	images := make([]string, 0, len(s.desired))
	for name, instance := range s.desired {
		rows = append(rows, s.containerRowLocked(name))
		images = append(images, instance.image)
	}
	s.mu.Unlock()

	for i, image := range images {
		whole, err := wholeReference(image)
		if err != nil {
			return nil, opererr.Wrap(err,
				"show container list could not complete the image of container "+rows[i].Name,
				"the configuration was accepted with an image the reference grammar refuses",
				runningConfigCommand,
			)
		}
		rows[i].Image = whole
	}
	slices.SortFunc(rows, func(a, b ContainerRow) int { return strings.Compare(a.Name, b.Name) })
	return rows, nil
}

// containerRowLocked answers the state of one configured container and the
// detail its state carries, with no image. MUST be called with s.mu held.
func (s *supervisor) containerRowLocked(name string) ContainerRow {
	row := ContainerRow{Name: name}
	state, known := s.states[name]
	if !known {
		// An enabled container no start ended for has left no state, and it
		// does not run.
		row.State = StateStopped.String()
		row.Reason = reasonNoStartYet
		return row
	}
	row.State = state.String()
	switch state {
	case StateRunning:
		if running, recorded := s.running[name]; recorded {
			row.Digest = running.digest
			row.Since = running.since
		}
	case StateWaitingImage:
		absent, recorded := s.waiting[name]
		if !recorded {
			// The present notice or a changed image ended the wait, and the
			// start that follows has not ended yet.
			row.Reason = reasonStartInProgress
			return row
		}
		row.Reason = oneLine(waitingReason(absent))
	case StateFailed:
		if detail, recorded := s.problems[problemRecord{name: name, kind: problemFailed}]; recorded {
			row.Reason = oneLine(detail.cause)
		}
	case StateUnspecified, StateRestarting, StateStopped, StateDisabled:
		// No record details these states yet: the restart policy, which
		// gives the restarting state its next start and the stopped state
		// its exit status, does not exist. No code sets StateUnspecified,
		// and its word shows if one ever does.
	}
	return row
}

// waitingReason answers why a container waits for its image, from the
// store's absent answer, naming the reference the store waits for.
func waitingReason(absent Absent) string {
	switch absent.Reason {
	case AbsentNeverFetched:
		return "its image " + absent.Reference + " was never fetched"
	case AbsentFetchRunning:
		return "the fetch of its image " + absent.Reference + " is running"
	case AbsentFetchFailed:
		return "the last fetch of its image " + absent.Reference + " failed: " + absent.Detail
	case AbsentImageLost:
		return "the image store no longer holds its image " + absent.Reference + " whole: " + absent.Detail
	case AbsentUnspecified:
		// No store answer carries it; the reference still says what the
		// container waits for.
	}
	return "its image " + absent.Reference + " is not in the image store"
}

// wholeReference answers the whole reference an image leaf completes to: the
// registry host, the repository and the tag or the digest.
func wholeReference(image string) (string, error) {
	reference, err := registry.ParseReference(image)
	if err != nil {
		return "", err
	}
	return reference.String(), nil
}

// oneLine joins the lines of text into one, so a row keeps one line in a
// table: a cause that quotes a multi-line error is shown whole.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
