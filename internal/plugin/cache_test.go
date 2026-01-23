package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/selector"
)

// mockReactorCache implements ReactorInterface for cache command testing.
type mockReactorCache struct {
	mockReactor
	retainCalled  bool
	releaseCalled bool
	deleteCalled  bool
	forwardCalled bool
	listCalled    bool
	lastID        uint64
	lastSelector  selector.Selector
	cachedIDs     []uint64
	retainErr     error
	releaseErr    error
	deleteErr     error
	forwardErr    error
}

func (m *mockReactorCache) RetainUpdate(id uint64) error {
	m.retainCalled = true
	m.lastID = id
	return m.retainErr
}

func (m *mockReactorCache) ReleaseUpdate(id uint64) error {
	m.releaseCalled = true
	m.lastID = id
	return m.releaseErr
}

func (m *mockReactorCache) DeleteUpdate(id uint64) error {
	m.deleteCalled = true
	m.lastID = id
	return m.deleteErr
}

func (m *mockReactorCache) ForwardUpdate(sel *selector.Selector, id uint64) error {
	m.forwardCalled = true
	m.lastID = id
	if sel != nil {
		m.lastSelector = *sel
	}
	return m.forwardErr
}

func (m *mockReactorCache) ListUpdates() []uint64 {
	m.listCalled = true
	return m.cachedIDs
}

// TestBgpCacheRetain verifies bgp cache <id> retain command.
//
// VALIDATES: Retain command parses ID and calls RetainUpdate.
// PREVENTS: Regression in cache retain functionality after migration.
func TestBgpCacheRetain(t *testing.T) {
	reactor := &mockReactorCache{}
	d := NewDispatcher()
	RegisterCacheHandlers(d)

	ctx := &CommandContext{
		Reactor:    reactor,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp cache 12345 retain")
	require.NoError(t, err)
	assert.Equal(t, statusDone, resp.Status)
	assert.True(t, reactor.retainCalled, "RetainUpdate should be called")
	assert.Equal(t, uint64(12345), reactor.lastID)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint64(12345), data["id"])
	assert.Equal(t, true, data["retained"])
}

// TestBgpCacheRelease verifies bgp cache <id> release command.
//
// VALIDATES: Release command parses ID and calls ReleaseUpdate.
// PREVENTS: Regression in cache release functionality after migration.
func TestBgpCacheRelease(t *testing.T) {
	reactor := &mockReactorCache{}
	d := NewDispatcher()
	RegisterCacheHandlers(d)

	ctx := &CommandContext{
		Reactor:    reactor,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp cache 99999 release")
	require.NoError(t, err)
	assert.Equal(t, statusDone, resp.Status)
	assert.True(t, reactor.releaseCalled, "ReleaseUpdate should be called")
	assert.Equal(t, uint64(99999), reactor.lastID)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint64(99999), data["id"])
	assert.Equal(t, true, data["released"])
}

// TestBgpCacheExpire verifies bgp cache <id> expire command.
//
// VALIDATES: Expire command parses ID and calls DeleteUpdate.
// PREVENTS: Regression in cache expire functionality after migration.
func TestBgpCacheExpire(t *testing.T) {
	reactor := &mockReactorCache{}
	d := NewDispatcher()
	RegisterCacheHandlers(d)

	ctx := &CommandContext{
		Reactor:    reactor,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp cache 55555 expire")
	require.NoError(t, err)
	assert.Equal(t, statusDone, resp.Status)
	assert.True(t, reactor.deleteCalled, "DeleteUpdate should be called")
	assert.Equal(t, uint64(55555), reactor.lastID)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint64(55555), data["id"])
	assert.Equal(t, true, data["expired"])
}

// TestBgpCacheList verifies bgp cache list command.
//
// VALIDATES: List command returns cached IDs.
// PREVENTS: Regression in cache list functionality after migration.
func TestBgpCacheList(t *testing.T) {
	reactor := &mockReactorCache{
		cachedIDs: []uint64{100, 200, 300},
	}
	d := NewDispatcher()
	RegisterCacheHandlers(d)

	ctx := &CommandContext{
		Reactor:    reactor,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp cache list")
	require.NoError(t, err)
	assert.Equal(t, statusDone, resp.Status)
	assert.True(t, reactor.listCalled, "ListUpdates should be called")

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []uint64{100, 200, 300}, data["ids"])
	assert.Equal(t, 3, data["count"])
}

// TestBgpCacheForward verifies bgp cache <id> forward <sel> command.
//
// VALIDATES: Forward command parses ID, selector and calls ForwardUpdate.
// PREVENTS: Regression in cache forward functionality after migration.
func TestBgpCacheForward(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  uint64
		wantSel string
		wantErr bool
	}{
		{
			name:    "forward_to_all",
			input:   "bgp cache 12345 forward *",
			wantID:  12345,
			wantSel: "*",
		},
		{
			name:    "forward_to_specific",
			input:   "bgp cache 12345 forward 10.0.0.1",
			wantID:  12345,
			wantSel: "10.0.0.1",
		},
		{
			name:    "forward_except",
			input:   "bgp cache 12345 forward !10.0.0.1",
			wantID:  12345,
			wantSel: "!10.0.0.1",
		},
		{
			name:    "missing_selector",
			input:   "bgp cache 12345 forward",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reactor := &mockReactorCache{}
			d := NewDispatcher()
			RegisterCacheHandlers(d)

			ctx := &CommandContext{
				Reactor:    reactor,
				Dispatcher: d,
			}

			resp, err := d.Dispatch(ctx, tt.input)

			if tt.wantErr {
				assert.Equal(t, statusError, resp.Status)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, statusDone, resp.Status)
			assert.True(t, reactor.forwardCalled, "ForwardUpdate should be called")
			assert.Equal(t, tt.wantID, reactor.lastID)
			assert.Equal(t, tt.wantSel, reactor.lastSelector.String())
		})
	}
}

// TestBgpCacheInvalidId verifies invalid cache ID handling.
//
// VALIDATES: Invalid IDs are rejected with appropriate error.
// PREVENTS: Panic or undefined behavior on malformed input.
func TestBgpCacheInvalidId(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"non_numeric", "bgp cache abc retain"},
		{"negative", "bgp cache -1 retain"},
		{"empty_id", "bgp cache retain"},
		{"overflow", "bgp cache 99999999999999999999 retain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reactor := &mockReactorCache{}
			d := NewDispatcher()
			RegisterCacheHandlers(d)

			ctx := &CommandContext{
				Reactor:    reactor,
				Dispatcher: d,
			}

			resp, _ := d.Dispatch(ctx, tt.input)
			assert.Equal(t, statusError, resp.Status, "should return error for: %s", tt.input)
			assert.False(t, reactor.retainCalled, "RetainUpdate should not be called")
		})
	}
}

// TestBgpCacheUnknownAction verifies unknown cache action handling.
//
// VALIDATES: Unknown actions are rejected.
// PREVENTS: Silent failures on typos.
func TestBgpCacheUnknownAction(t *testing.T) {
	reactor := &mockReactorCache{}
	d := NewDispatcher()
	RegisterCacheHandlers(d)

	ctx := &CommandContext{
		Reactor:    reactor,
		Dispatcher: d,
	}

	resp, _ := d.Dispatch(ctx, "bgp cache 12345 unknown")
	assert.Equal(t, statusError, resp.Status)
}

// TestBgpCacheHelp verifies bgp cache returns help on no args.
//
// VALIDATES: Bare "bgp cache" returns usage help.
func TestBgpCacheHelp(t *testing.T) {
	reactor := &mockReactorCache{}
	d := NewDispatcher()
	RegisterCacheHandlers(d)

	ctx := &CommandContext{
		Reactor:    reactor,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp cache")
	require.NoError(t, err)
	// Should return help/usage, not an error
	assert.Equal(t, statusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	_, hasCommands := data["commands"]
	assert.True(t, hasCommands, "should return available commands")
}
