// Design: docs/architecture/web-interface.md -- native forms over shared artifact editing.
package web

import (
	"bytes"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config/artifact"
)

type backupPageData struct {
	Path     string
	Label    string
	Names    []string
	Selected string
	Content  string
	Base     string
	Review   string
	Diff     string
	Status   string
	Error    string
	CanSave  bool
	Source   *backupSourcePageData
}

func (m *backupWorkspaces) page(w http.ResponseWriter, data backupPageData, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := writeComponent(w, "backup workspace", pageBackupWorkspace(data)); err != nil {
		reportBackupStorageFailure()
	}
}

func (m *backupWorkspaces) config(r *http.Request, id, name string) (string, error) {
	transfer, err := m.command(r.Context(), r, "request config artifact source begin artifact "+backupWord(id)+" name "+backupWord(name))
	if err != nil {
		return "", err
	}
	var content bytes.Buffer
	if err := m.read(r, transfer.Transfer, &content); err != nil {
		return "", err
	}
	return content.String(), nil
}

func (m *backupWorkspaces) edit(w http.ResponseWriter, r *http.Request, id string) {
	info, err := m.command(r.Context(), r, "show config artifact info artifact "+backupWord(id))
	if err != nil {
		backupCommandFailure(w, err)
		return
	}
	selected := r.URL.Query().Get("config")
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, backupTextBytesMax*3+(64<<10))
		if err := r.ParseForm(); err != nil {
			backupRefuse(w, http.StatusBadRequest, "The backup edit form could not be read.", "The submitted form is malformed or exceeds its size allowance; the artifact was not changed.", "Open the backup workspace again and submit at most 4 MiB of configuration text")
			return
		}
		if len(r.PostForm) > 5 {
			backupFormRefusal(w)
			return
		}
		for _, values := range r.PostForm {
			if len(values) != 1 {
				backupFormRefusal(w)
				return
			}
		}
		if r.PostForm.Get("action") == "close" {
			if _, err := m.command(r.Context(), r, "delete config artifact "+backupWord(id)); err != nil {
				backupCommandFailure(w, err)
				return
			}
			http.Redirect(w, r, backupWorkspacePath, http.StatusSeeOther)
			return
		}
		selected = r.PostForm.Get("config")
	}
	if selected == "" {
		if len(info.Names) == 0 {
			backupMissing(w)
			return
		}
		selected = info.Names[0]
	}
	if !slices.Contains(info.Names, selected) {
		backupMissing(w)
		return
	}
	original, err := m.config(r, id, selected)
	if err != nil {
		backupCommandFailure(w, err)
		return
	}
	data := backupPageData{Path: backupWorkspaceURL(id), Label: info.Label, Names: info.Names, Selected: selected, Content: original, Base: backupDigest([]byte(original))}
	if r.Method == http.MethodGet {
		m.page(w, data, http.StatusOK)
		return
	}
	data.Content = r.PostForm.Get("content")
	if len(data.Content) > backupTextBytesMax {
		backupRefuse(w, http.StatusRequestEntityTooLarge, "The submitted configuration exceeds 4 MiB.", "The artifact was not changed.", "Reduce the configuration text or edit the artifact with ze config edit --backup")
		return
	}
	action := r.PostForm.Get("action")
	if action != "validate" {
		if action != "review" {
			if action != verbSave {
				backupFormRefusal(w)
				return
			}
		}
	}
	text, err := m.send(r, "request config artifact text begin total "+strconv.Itoa(len(data.Content)), strings.NewReader(data.Content))
	if err != nil {
		backupCommandFailure(w, err)
		return
	}
	defer func() {
		if err := m.abort(r, text.Transfer); err != nil {
			reportBackupStorageFailure()
		}
	}()
	line := "request config artifact " + action + " artifact " + backupWord(id) + " name " + backupWord(selected) + " text " + backupWord(text.Transfer) + " base " + backupWord(r.PostForm.Get("base"))
	if action == "save" {
		line += " review " + backupWord(r.PostForm.Get("review"))
	}
	answer, err := m.command(r.Context(), r, line)
	if err != nil {
		data.Error = err.Error()
		status := http.StatusInternalServerError
		if refusal := backupSingleCommandError(err); refusal != nil {
			status = refusal.status
		}
		m.page(w, data, status)
		return
	}
	data.Base = answer.Base
	data.Review = answer.Review
	data.Diff = answer.Diff
	data.Status = answer.Message
	if answer.DiffTransfer != "" {
		var diff bytes.Buffer
		if err := m.read(r, answer.DiffTransfer, &diff); err != nil {
			backupCommandFailure(w, err)
			return
		}
		data.Diff = diff.String()
	}
	data.CanSave = answer.Review != ""
	if answer.Saved {
		data.Content, err = m.config(r, id, selected)
		if err != nil {
			backupCommandFailure(w, err)
			return
		}
	}
	m.page(w, data, http.StatusOK)
}

func backupFormRefusal(w http.ResponseWriter) {
	backupRefuse(w, http.StatusBadRequest, "This backup edit form is not recognized.", "The artifact was not changed.", "Open the backup workspace again and use its Validate, Review changes or Save into backup button")
}
func backupDigest(content []byte) string            { return artifact.Digest(content) }
func backupReviewToken(base, content string) string { return artifact.ReviewToken(base, content) }
