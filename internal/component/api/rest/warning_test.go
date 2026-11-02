package rest

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestExecResultJSONCarriesWarnings writes a response that warns about its
// answer the way POST /api/v1/execute does, and decodes the body a client
// reads.
//
// VALIDATES: the JSON body holds "warnings", every line `ze <command>`
// prints on stderr, beside "status" and "data"; an answer that warns of
// nothing carries no "warnings" key.
// PREVENTS: the REST surface dropping a warning the other surfaces show.
func TestExecResultJSONCarriesWarnings(t *testing.T) {
	rec := httptest.NewRecorder()
	writeExecResult(rec, &api.ExecResult{
		Status:   plugin.StatusWarning,
		Data:     plugin.Map{"answer": "kept"},
		Warnings: []string{"warning: first line", "warning: second line"},
	})
	var body struct {
		Status   string            `json:"status"`
		Data     map[string]string `json:"data"`
		Warnings []string          `json:"warnings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if body.Status != plugin.StatusWarning || body.Data["answer"] != "kept" {
		t.Errorf("status %q data %v, want the warned answer", body.Status, body.Data)
	}
	if len(body.Warnings) != 2 || body.Warnings[0] != "warning: first line" || body.Warnings[1] != "warning: second line" {
		t.Errorf("warnings %q, want both lines", body.Warnings)
	}

	quiet := httptest.NewRecorder()
	writeExecResult(quiet, &api.ExecResult{Status: plugin.StatusDone, Data: plugin.Map{"answer": "kept"}})
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(quiet.Body.Bytes(), &fields); err != nil {
		t.Fatalf("decode %q: %v", quiet.Body.String(), err)
	}
	if _, present := fields["warnings"]; present {
		t.Errorf("an answer that warns of nothing carries a warnings key: %s", quiet.Body.String())
	}
}
