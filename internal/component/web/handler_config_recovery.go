// Design: docs/architecture/web-interface.md -- authorized recovery of corrupt pending changes.
// Related: editor_change_recovery.go in cli -- guarded inspection and removal.
package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/opererr"
)

const configRecoveryPath = "/config/recover"

var (
	errConfigRecoveryChanged = opererr.New("Your pending change file changed after you opened recovery.",
		"No pending changes were discarded; review the current file before confirming again", "Open /config/recover")
	errConfigRecoveryBusy = opererr.New("Your configuration editor is busy with another request.",
		"No pending changes were discarded; wait for that request to finish before trying recovery again", "Open /config/recover")
)

type configRecoveryData struct {
	Username    string
	Fingerprint string
}

type configRecoveryErrorData struct {
	ChangeName string
}

// HandleConfigRecovery opens no candidate: recovery must remain reachable when
// the normal editor cannot parse the authenticated account's pending changes.
// The caller MUST authenticate requests and apply same-origin checks to POST.
func HandleConfigRecovery(mgr *EditorManager, renderer *Renderer, authorizer aaa.Authorizer, recorder audit.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, opererr.New("Sign in before recovering pending changes.",
				"Recovery requires an authenticated account with discard permission",
				"Open /login").Error(), http.StatusUnauthorized)
			return
		}
		if !authorizeWebConfigMutation(w, r, authorizer, username, webCommandConfigDiscard) {
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		switch r.Method {
		case http.MethodGet:
			handleConfigRecoveryGet(w, r, mgr, renderer, username)
		case http.MethodPost:
			handleConfigRecoveryPost(w, r, mgr, username, recorder)
		default:
			http.Error(w, opererr.New("This recovery request used an unsupported method.",
				"No pending changes were discarded", "Open /config/recover").Error(), http.StatusMethodNotAllowed)
		}
	}
}

func handleConfigRecoveryGet(w http.ResponseWriter, r *http.Request, mgr *EditorManager, renderer *Renderer, username string) {
	recovery, err := cli.InspectCorruptChangeFile(mgr.store, mgr.configPath, mgr.schema, username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var content bytes.Buffer
	if err := configRecovery(configRecoveryData{Username: username, Fingerprint: recovery.Fingerprint()}).Render(r.Context(), &content); err != nil {
		writeConfigRecoveryDisplayFailure(w, err)
		return
	}
	if err := renderer.RenderLayout(w, LayoutData{
		Title: "Recover pending changes", Username: username, HasSession: true,
		Content: template.HTML(content.String()), ReadOnly: true, ActiveUI: uiModeTokenFinder, //nolint:gosec // trusted templ output
	}); err != nil {
		writeConfigRecoveryDisplayFailure(w, err)
	}
}

func handleConfigRecoveryPost(w http.ResponseWriter, r *http.Request, mgr *EditorManager, username string, recorder audit.Recorder) {
	fingerprint, err := parseConfigRecoveryConfirmation(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	outcome, err := mgr.recoverCorruptChangeFile(username, fingerprint)
	// Removal is the audited action, even if refreshing the cached editor then
	// finds a separate fault. Do not tell the operator to repeat that removal.
	switch outcome {
	case cli.ChangeRecoveryKept:
	case cli.ChangeRecoveryRemoved:
		recordWebAudit(recorder, r, username, audit.ActionConfigDiscard,
			"Recovered unreadable pending change file for account "+username+
				"; discarded changes and structural instructions in this file from all origins; sha256 "+fingerprint+
				"; saved draft file and running configuration preserved")
	case cli.ChangeRecoveryUncertain:
		recordConfigRecoveryUncertainty(recorder, r, username, fingerprint)
	case cli.ChangeRecoveryUnspecified:
		panic("BUG: pending-change recovery returned an unspecified outcome")
	default:
		panic("BUG: pending-change recovery returned an unknown outcome")
	}
	if errors.Is(err, errConfigRecoveryChanged) {
		http.Error(w, configRecoveryRefusal(err).Error(), http.StatusConflict)
		return
	}
	if errors.Is(err, errConfigRecoveryBusy) {
		http.Error(w, configRecoveryRefusal(err).Error(), http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, configRecoveryRefusal(err).Error(), http.StatusInternalServerError)
		return
	}
	htmxRedirect(w, r, "/show/")
}

// recoverCorruptChangeFile returns the storage outcome independently of a later
// editor refresh error. Safe for concurrent use. The existing cached editor is
// refreshed in place because queued requests can hold its pointer.
func (m *EditorManager) recoverCorruptChangeFile(username, fingerprint string) (cli.ChangeRecoveryOutcome, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	us := m.sessions[username]
	if us != nil {
		// Commit holds the user lock before reading the manager. Waiting for
		// that lock here would invert the order and deadlock both requests.
		if !us.mu.TryLock() {
			return cli.ChangeRecoveryKept, errConfigRecoveryBusy
		}
		defer us.mu.Unlock()
	}
	recovery, err := cli.InspectCorruptChangeFile(m.store, m.configPath, m.schema, username)
	if err != nil {
		return cli.ChangeRecoveryKept, err
	}
	if recovery.Fingerprint() != fingerprint {
		return cli.ChangeRecoveryKept, errConfigRecoveryChanged
	}
	outcome, discardErr := recovery.Discard()
	switch outcome {
	case cli.ChangeRecoveryKept:
		return outcome, discardErr
	case cli.ChangeRecoveryRemoved:
	case cli.ChangeRecoveryUncertain:
		// Re-reading uncertain removal through ResumeSession could parse the
		// very file recovery could not inspect, exposing its raw parser error.
		if us != nil {
			us.editor = blockedRecoveryEditor(discardErr)
			delete(m.sessions, username)
		}
		return outcome, discardErr
	case cli.ChangeRecoveryUnspecified:
		panic("BUG: pending-change removal returned an unspecified outcome")
	default:
		panic("BUG: pending-change removal returned an unknown outcome")
	}
	if us != nil {
		if err := us.editor.ResumeSession(); err != nil {
			refusal := configRecoveryRefreshFailure(errors.Join(discardErr, err))
			us.editor = blockedRecoveryEditor(refusal)
			delete(m.sessions, username)
			return outcome, refusal
		}
	}
	return outcome, discardErr
}

func configRecoveryRefreshFailure(err error) *opererr.Error {
	if cli.IsCorruptChangeFile(err) {
		// A concurrent session can replace the removed file before resumption.
		// Its parser error can contain secrets; report the phase without it.
		return opererr.New("Your unreadable pending change file was discarded, but the editor could not be reopened.",
			"A pending change file is still unreadable; no replacement file was discarded. The separate saved draft file and running configuration were kept",
			"Open /config/recover")
	}
	return opererr.Wrap(err,
		"Your unreadable pending change file was discarded, but the editor could not be reopened.",
		"The saved draft file and running configuration were kept; instructions in the removed pending change file were discarded. Repair the remaining configuration or storage error before editing", "Reload /show/")
}

// configRecoveryRefusal preserves the producer's known or uncertain outcome.
// Candidate-opening refusals must not replace that result with a retention claim.
func configRecoveryRefusal(err error) *opererr.Error {
	if refusal, ok := errors.AsType[*opererr.Error](err); ok {
		return refusal
	}
	return opererr.Wrap(err, "Pending-change recovery could not be completed.",
		"The recovery result could not be established; running configuration was not changed",
		"Open /config/recover")
}

func recordConfigRecoveryUncertainty(recorder audit.Recorder, r *http.Request, username, fingerprint string) {
	if recorder == nil {
		return
	}
	if err := recorder.Record(audit.Entry{
		Actor: username, RemoteAddr: r.RemoteAddr, Surface: audit.Web,
		Action: audit.ActionConfigDiscard, Outcome: audit.OutcomeError,
		Detail: "Attempted recovery of unreadable pending change file for account " + username +
			"; removal outcome could not be determined; sha256 " + fingerprint,
	}); err != nil {
		serverLogger.Warn("audit record failed", "code", "doctor-audit-record-failed", "action", audit.ActionConfigDiscard, "user", username, "error", err)
	}
}

func parseConfigRecoveryConfirmation(w http.ResponseWriter, r *http.Request) (string, error) {
	refusal := opererr.New("The pending-change recovery confirmation is incomplete or invalid.",
		"No pending changes were discarded; use the recovery form and select its confirmation checkbox",
		"Open /config/recover")
	if r.URL.RawQuery != "" {
		return "", refusal
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		return "", refusal
	}
	for name, values := range r.PostForm {
		switch name {
		case "confirm", "fingerprint":
		default:
			return "", refusal
		}
		if len(values) != 1 {
			return "", refusal
		}
	}
	if r.PostForm.Get("confirm") != "discard" {
		return "", refusal
	}
	fingerprint := r.PostForm.Get("fingerprint")
	if len(fingerprint) != sha256.Size*2 {
		return "", refusal
	}
	if _, err := hex.DecodeString(fingerprint); err != nil {
		return "", refusal
	}
	return fingerprint, nil
}

// writeConfigRecoveryError names the retained file from the manager and the
// authenticated account whose editor open failed. It never renders the parser
// error: a malformed assignment can put a secret there before masking can identify it.
func writeConfigRecoveryError(w http.ResponseWriter, r *http.Request, renderer *Renderer, mgr *EditorManager) {
	view := configRecoveryErrorData{
		ChangeName: filepath.Base(cli.ChangePath(mgr.configPath, GetUsernameFromRequest(r))),
	}
	if r.Header.Get("HX-Request") == htmxRequestTrue {
		if err := writeComponent(w, "pending-change recovery error", configRecoveryError(view)); err != nil {
			writeConfigRecoveryDisplayFailure(w, err)
		}
		return
	}
	var content bytes.Buffer
	if err := configRecoveryError(view).Render(r.Context(), &content); err != nil {
		writeConfigRecoveryDisplayFailure(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	if err := renderer.RenderLayout(w, LayoutData{
		Title: "Pending changes need recovery", Username: GetUsernameFromRequest(r), HasSession: true,
		Content: template.HTML(content.String()), ReadOnly: true, ActiveUI: uiModeTokenFinder, //nolint:gosec // trusted templ output
	}); err != nil {
		writeConfigRecoveryDisplayFailure(w, err)
	}
}

func writeConfigRecoveryDisplayFailure(w http.ResponseWriter, err error) {
	http.Error(w, opererr.Wrap(err, "The pending-change recovery page could not be displayed.",
		"No recovery action was submitted", "Open /config/recover").Error(), http.StatusInternalServerError)
}
