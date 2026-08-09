// Design: docs/architecture/config/system-update.md -- gokrazy-managed update backend

package system

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sort"
	"time"

	"github.com/ze-software/ze/internal/core/gokrazyutil"
)

const (
	managedStatus  = "managed by gokrazy"
	managedMessage = "system image updates are managed by gokrazy; use the gokrazy management interface"
	probeTimeout   = 2 * time.Second
	probeMaxBody   = 64 << 10
)

func init() {
	registerBackend(BackendGokrazyAB, newGokrazyBackend)
}

type gokrazyBackend struct {
	client *http.Client
}

type probeResult struct {
	reachable bool
	features  []string
}

func newGokrazyBackend(_ UpdateCheckConfig, opts BackendOptions) (UpdateBackend, error) {
	socketPath := opts.GokrazySocketPath
	if socketPath == "" {
		socketPath = gokrazyutil.DefaultSocketPath
	}
	backend := &gokrazyBackend{}
	backend.client = &http.Client{
		Timeout: probeTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
	}
	return backend, nil
}

func (b *gokrazyBackend) Name() BackendName { return BackendGokrazyAB }

func (b *gokrazyBackend) Start(context.Context) {}

func (b *gokrazyBackend) Stop() {}

func (b *gokrazyBackend) Status() ExtendedUpdateStatus {
	probe := b.probe()
	status := ExtendedUpdateStatus{
		Backend:          BackendGokrazyAB,
		StatusText:       managedStatus,
		Message:          managedMessage,
		GokrazyReachable: probe.reachable,
		GokrazyFeatures:  probe.features,
	}
	if !probe.reachable {
		status.LastError = "gokrazy management unavailable"
	}
	return status
}

func (b *gokrazyBackend) Check(context.Context) (ExtendedUpdateStatus, error) {
	return b.Status(), ErrFirmwareUnsupported
}

func (b *gokrazyBackend) Download(context.Context) (FirmwareResult, error) {
	return UnsupportedResult(BackendGokrazyAB), ErrFirmwareUnsupported
}

func (b *gokrazyBackend) Apply(context.Context) (FirmwareResult, error) {
	return UnsupportedResult(BackendGokrazyAB), ErrFirmwareUnsupported
}

func (b *gokrazyBackend) Restart() (FirmwareResult, error) {
	return UnsupportedResult(BackendGokrazyAB), ErrFirmwareUnsupported
}

func (b *gokrazyBackend) Rollback() (FirmwareResult, error) {
	return UnsupportedResult(BackendGokrazyAB), ErrFirmwareUnsupported
}

func (b *gokrazyBackend) History() []UpdateEvent { return nil }

func (b *gokrazyBackend) probe() probeResult {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	features, err := b.getJSON(ctx, "/update/features")
	if err == nil {
		return probeResult{reachable: true, features: featureNames(features)}
	}
	if err := b.getOK(ctx, "/"); err == nil {
		return probeResult{reachable: true}
	}
	return probeResult{}
}

func (b *gokrazyBackend) getOK(ctx context.Context, path string) error {
	resp, err := b.doGet(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // probe path closes best-effort
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return errors.New("unexpected gokrazy status")
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, probeMaxBody))
	return nil
}

func (b *gokrazyBackend) getJSON(ctx context.Context, path string) (any, error) {
	resp, err := b.doGet(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // probe path closes best-effort
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New("unexpected gokrazy status")
	}
	var out any
	dec := json.NewDecoder(io.LimitReader(resp.Body, probeMaxBody))
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (b *gokrazyBackend) doGet(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://gokrazy"+path, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if auth := gokrazyutil.AuthHeader(); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return b.client.Do(req)
}

func featureNames(raw any) []string {
	features := make([]string, 0)
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if name, ok := item.(string); ok && name != "" {
				features = append(features, name)
			}
		}
	case map[string]any:
		for name, value := range v {
			if name != "" && featureEnabled(value) {
				features = append(features, name)
			}
		}
	}
	sort.Strings(features)
	return features
}

func featureEnabled(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return value != nil
	}
}
