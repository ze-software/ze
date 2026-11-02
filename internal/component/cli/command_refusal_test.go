package cli

import (
	"errors"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestFeedbackLineShowsTheRefusalAsWritten checks how an interactive session
// shows a command's refusal.
//
// VALIDATES: a refusal the command wrote whole is shown as `ze <command>`
// prints it, with no "Error: " in front; any other error keeps the session's
// "Error: " rendering.
// PREVENTS: the SSH session reading "Error: usage: ..." where the local route
// reads "usage: ...".
func TestFeedbackLineShowsTheRefusalAsWritten(t *testing.T) {
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = colorprofile.Ascii })

	refused := Model{err: plugin.NewRefusalError("usage: show config list\n")}
	assert.Equal(t, errorStyle.Render("usage: show config list"), refused.feedbackLine())

	plain := Model{err: errors.New("plain failure")}
	assert.Equal(t, errorStyle.Render("Error: plain failure"), plain.feedbackLine())
}
