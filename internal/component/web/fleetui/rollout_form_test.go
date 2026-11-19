package fleetui

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// rolloutStartTargets answers the confirmation line Start carries for a
// preview: what the operator is asked to confirm.
func rolloutStartTargets(preview *RolloutPreviewView) string {
	return `data-confirm-targets="` + strconv.Itoa(preview.Connected) + " connected devices in " +
		strconv.Itoa(len(preview.Batches)) + ` batches"`
}

// TestRolloutPreviewStartsTheChosenBatches asks the preview fragment the
// batch size select swaps in, for two sizes other than the default.
//
// VALIDATES: the Start action is drawn with the preview it confirms, so the
// confirmation names the batches of the size the operator chose.
// PREVENTS: a Start drawn once with the page, which keeps confirming the
// default batch count after the operator chose another size.
func TestRolloutPreviewStartsTheChosenBatches(t *testing.T) {
	request := manageRequest(t, "/fleet/estate/rollouts/?sub="+newRolloutSub)

	for _, percent := range []int{10, 100} {
		preview, err := rolloutPreview(request.Snapshot.Views, request.Address.View.Filter, percent)
		require.NoError(t, err)
		require.NotEmpty(t, preview.Batches, "the test estate has connected devices to split")

		component, err := rolloutPreviewFragment(context.Background(), request, strconv.Itoa(percent))
		require.NoError(t, err)
		markup := componentMarkup(t, component)

		require.Contains(t, markup, `data-confirm-action="Start the rollout"`, "the preview at %d%% draws the Start it confirms", percent)
		require.Contains(t, markup, rolloutStartTargets(&preview), "Start at %d%% confirms that preview's batches", percent)
	}

	ten, err := rolloutPreview(request.Snapshot.Views, request.Address.View.Filter, 10)
	require.NoError(t, err)
	whole, err := rolloutPreview(request.Snapshot.Views, request.Address.View.Filter, 100)
	require.NoError(t, err)
	require.NotEqual(t, len(ten.Batches), len(whole.Batches), "the two sizes must split differently for the test to judge anything")
}

// TestRolloutFormPostsItsFields renders the new rollout form.
//
// VALIDATES: the form Start submits includes the fields of the new rollout
// form (what to push, the batch size, the threshold, the gate), and Start
// sits inside the preview the batch size select swaps.
// PREVENTS: a Start that posts only its own name, so the rollout started is
// not the one the operator chose.
func TestRolloutFormPostsItsFields(t *testing.T) {
	request := manageRequest(t, "/fleet/estate/rollouts/?sub="+newRolloutSub)
	component, err := rolloutForm(context.Background(), request)
	require.NoError(t, err)
	markup := componentMarkup(t, component)

	require.Contains(t, markup, `id="`+rolloutNewFormID+`"`)
	require.Contains(t, markup, `hx-include="#`+rolloutNewFormID+`"`, "the manage form posts the new rollout form's fields")

	preview := strings.Index(markup, `id="fleet-rollout-preview"`)
	start := strings.Index(markup, `data-confirm-action="Start the rollout"`)
	require.GreaterOrEqual(t, preview, 0)
	require.Greater(t, start, preview, "Start is drawn inside the preview the batch size swaps")
	button := strings.LastIndex(markup[:start], "<button")
	require.GreaterOrEqual(t, button, 0)
	require.Contains(t, markup[button:start], `form="`+manageActionFormID+`"`, "Start submits the manage form")
}

// TestRolloutGroupTargetsNameTheGroupsDevices writes the filter of a group
// target for levels that read a field and levels that read a label, keyed
// like a field, like the word label and like the word has.
//
// VALIDATES: a group of a registered tree level is named by its field term,
// and a group of a label level by the label term the Label menu writes, so
// the target holds the devices of the group the link names.
// PREVENTS: a label level written as key:value, which reads the label model
// as the model field, the label has as a flag test and the label label as
// another pair, so the rollout targets other devices than the group.
func TestRolloutGroupTargetsNameTheGroupsDevices(t *testing.T) {
	cases := []struct {
		level GroupLevel
		want  string
	}{
		{GroupLevel{Key: fieldState, Value: "offline"}, "state:offline"},
		{GroupLevel{Key: "site", Value: "lon"}, "site:lon"},
		{GroupLevel{Key: fieldModel, Value: "mx"}, "label:model=mx"},
		{GroupLevel{Key: "has", Value: "alarm"}, "label:has=alarm"},
		{GroupLevel{Key: "label", Value: "site=lon"}, "label:label=site=lon"},
	}
	for _, tc := range cases {
		got, carried := groupFilter(tc.level)
		require.True(t, carried, "%s=%s", tc.level.Key, tc.level.Value)
		require.Equal(t, tc.want, got, "the group %s=%s", tc.level.Key, tc.level.Value)
	}
}
