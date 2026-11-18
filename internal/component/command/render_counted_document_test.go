// Design: docs/architecture/api/ipc_protocol.md -- counted document rendering.
package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A new count result must not wrap the sum of selected and retained refused
// records. These are opaque input counts, never proof of native ownership.
func TestCountedDocumentRefusesOutcomeOverflow(t *testing.T) {
	rendered, answer, err := RenderCountedDocument("show cleanup | count | json", "",
		`{"cleanup":[{"id":1},{"id":2}]}`, 0, ^uint64(0))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceed the answer counter limit")
	assert.Contains(t, err.Error(), "ze show errors")
	assert.Empty(t, rendered)
	assert.Empty(t, answer.Type, "refused rendering gained a successful answer descriptor")
	assert.Zero(t, answer.Count)
	assert.Zero(t, answer.Faults)
}

// Presentation alone retains the supplied original counts, independently of
// the document's visible collection length. Raw rendering also preserves bytes.
func TestCountedDocumentPresentationKeepsOriginalOutcome(t *testing.T) {
	const document = `{"cleanup": [{"id":1}, {"id":2}]}`
	for _, format := range []string{"raw", "json"} {
		t.Run(format, func(t *testing.T) {
			rendered, answer, err := RenderCountedDocument("show cleanup | "+format, "", document, 7, 3)
			require.NoError(t, err)
			assert.Equal(t, rpc.AnswerTypeDocument, answer.Type)
			assert.Equal(t, uint64(7), answer.Count)
			assert.Equal(t, uint64(3), answer.Faults)
			if format == "raw" {
				assert.Equal(t, document, rendered)
				return
			}
			assert.JSONEq(t, `[{"id":1},{"id":2}]`, rendered)
		})
	}
}

// Preparation and evaluation failures return their reason without a rendered
// body or a successful descriptor. The transport has not been asked to write.
func TestCountedDocumentRefusalHasNoRenderedPayload(t *testing.T) {
	for _, question := range []struct {
		name     string
		command  string
		document string
		problem  string
	}{
		{name: "invalid argument", command: "show cleanup | first nope", document: `{"cleanup":[]}`, problem: "first"},
		{name: "unsupported shape", command: "show cleanup | count", document: `{"label":"no rows"}`, problem: "count needs rows"},
	} {
		t.Run(question.name, func(t *testing.T) {
			rendered, answer, err := RenderCountedDocument(question.command, "", question.document, 7, 0)
			require.Error(t, err)
			assert.Contains(t, err.Error(), question.problem)
			assert.Empty(t, rendered)
			assert.Empty(t, answer.Type)
			assert.Zero(t, answer.Count)
			assert.Zero(t, answer.Faults)
		})
	}
}
