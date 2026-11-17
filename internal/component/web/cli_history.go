// Design: docs/architecture/web-interface.md -- one submission recorder for browser CLI.
// Related: cli_terminal.go, cli_stream.go -- finite fallback shares request ownership.
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/core/opererr"
)

const historyWarningHeader = "X-Ze-History-Warning"
const historyReceiptMax = 128

type historySubmissionKey struct{}

// cliHistorySubmission is request-local. The frozen matcher is also consumed
// by execution/authz, so a registry change cannot reclassify this command.
type cliHistorySubmission struct {
	registered func(string) bool
	warning    string
}

// frozenCLIRegistration MUST feed both history and execution for one request.
// A fallback request already carries the stream adapter's frozen answer.
func frozenCLIRegistration(r *http.Request, input string, registered func(string) bool) func(string) bool {
	if submission, ok := r.Context().Value(historySubmissionKey{}).(cliHistorySubmission); ok {
		return submission.registered
	}
	if registered == nil {
		return nil
	}
	matched := registered(input)
	return func(string) bool { return matched }
}

// recordCLIHistory is the single adapter used by all three submission routes.
// The caller MUST already bound/parse the form and decode the execution path.
// It records command text only and marks even a failed admission so finite
// fallback cannot record twice. Recall failure never prevents command execution.
func recordCLIHistory(w http.ResponseWriter, r *http.Request, mgr *EditorManager, mode string, path []string, input string, registered func(string) bool) *http.Request {
	if _, recorded := r.Context().Value(historySubmissionKey{}).(cliHistorySubmission); recorded {
		return r
	}
	submission := cliHistorySubmission{registered: registered}
	if mgr != nil {
		start := cli.ModeConfig
		if mode == terminalModeOperational {
			start = cli.ModeOperational
		}
		effective, record := cli.HistoryCommandMode(start, input, registered)
		if r.FormValue("history") == "0" {
			record = false // The browser's initial automatic show is a view refresh, not Enter.
		}
		if record {
			receipt := r.FormValue("submission")
			var err error
			if validHistoryReceipt(receipt) {
				err = mgr.RecordHistory(GetUsernameFromRequest(r), effective, path, input, receipt)
			} else {
				err = historyUnavailable("The command recall submission identifier is invalid.")
			}
			if err != nil {
				submission.warning = err.Error()
				w.Header().Set(historyWarningHeader, historyWarningValue(submission.warning))
				reportBrowserHistoryUnavailable()
			}
		}
	}
	return r.WithContext(context.WithValue(r.Context(), historySubmissionKey{}, submission))
}

func validHistoryReceipt(receipt string) bool {
	if len(receipt) > historyReceiptMax {
		return false
	}
	for _, char := range receipt {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_", char) {
			return false
		}
	}
	return true
}

// submittedCLIMode accepts the two exact protocol spellings. Legacy requests
// without a mode keep the configuration bar's existing default.
func submittedCLIMode(w http.ResponseWriter, r *http.Request) (string, bool) {
	mode := r.FormValue("mode")
	switch mode {
	case "", terminalModeConfig:
		return terminalModeConfig, true
	case terminalModeOperational:
		return terminalModeOperational, true
	default:
		http.Error(w, opererr.New("The submitted CLI mode is invalid.",
			"The command was not run because its mode is neither configuration nor operational.",
			"Open /cli and select configuration or operational mode").Error(), http.StatusBadRequest)
		return "", false
	}
}

func cliHistoryWarning(r *http.Request) string {
	submission, _ := r.Context().Value(historySubmissionKey{}).(cliHistorySubmission)
	return submission.warning
}

func writeCLIHistory(w http.ResponseWriter, r *http.Request, mgr *EditorManager) {
	w.Header().Set("Cache-Control", "private, no-store")
	username := GetUsernameFromRequest(r)
	if username == "" {
		http.Error(w, opererr.New("Your browser session is not signed in.",
			"Command recall is private to the signed-in user.",
			"Open /login and sign in, then reopen /cli").Error(), http.StatusUnauthorized)
		return
	}
	var mode cli.EditorMode
	switch r.URL.Query().Get("mode") {
	case terminalModeConfig:
		mode = cli.ModeConfig
	case terminalModeOperational:
		mode = cli.ModeOperational
	default:
		http.Error(w, opererr.New("The command history mode is invalid.", "Recall belongs to configuration or operational mode.", "Open /cli and select configuration or operational mode").Error(), http.StatusBadRequest)
		return
	}
	entries, err := mgr.HistorySnapshot(username, mode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if entries == nil {
		entries = []string{}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(struct {
		Entries []string `json:"entries"`
	}{Entries: entries}); err != nil {
		reportBrowserHistoryResponseFailure()
	}
}

// historyWarningValue strips line breaks at the HTTP header boundary. The full
// structured warning remains in terminal JSON; a header must stay one line.
func historyWarningValue(warning string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(warning)
}
