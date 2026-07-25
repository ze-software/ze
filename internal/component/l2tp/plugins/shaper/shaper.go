// Design: docs/research/l2tpv2-ze-integration.md -- l2tp-shaper event handlers

package l2tpshaper

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/pkg/ze"
)

var errNoTrafficBackendLoadedConfigureTraffic = errors.New("no traffic backend loaded; configure traffic-control or wait for it to start")

type shaperPlugin struct {
	cfgPtr   atomic.Pointer[shaperConfig]
	sessions sync.Map // sessionKey -> sessionState

	busMu sync.Mutex
	bus   ze.EventBus
	unsub func()
}

type sessionKey struct {
	tunnelID  uint16
	sessionID uint16
}

type sessionState struct {
	iface        string
	downloadRate uint64
	uploadRate   uint64
	appliedAt    time.Time
}

var shaperInstance = &shaperPlugin{}

func (s *shaperPlugin) setEventBus(eb ze.EventBus) {
	s.busMu.Lock()
	defer s.busMu.Unlock()
	if s.unsub != nil {
		s.unsub()
	}
	s.bus = eb

	unsubUp := l2tpevents.SessionUp.Subscribe(eb, s.onSessionUp)
	unsubDown := l2tpevents.SessionDown.Subscribe(eb, s.onSessionDown)
	unsubRate := l2tpevents.SessionRateChange.Subscribe(eb, s.onSessionRateChange)
	s.unsub = func() {
		unsubUp()
		unsubDown()
		unsubRate()
	}
}

// handleSubscriberSessionUp is the subscriber.ShaperHandler registered
// at init. Called by PPPoE on SessionUp. Applies TC with the default
// configured rate (RADIUS metadata is L2TP-only for now).
func (s *shaperPlugin) handleSubscriberSessionUp(iface string, downloadRate, _ uint64) {
	cfg := s.cfgPtr.Load()
	if cfg == nil {
		return
	}
	if downloadRate == 0 {
		downloadRate = cfg.DefaultRate
	}
	if err := s.applyTC(iface, cfg.QdiscType, downloadRate); err != nil {
		logger().Warn("l2tp-shaper: failed to apply TC on subscriber session-up",
			"interface", iface, "error", err)
		return
	}
	logger().Info("l2tp-shaper: applied shaping (subscriber)",
		"interface", iface, "rate-bps", downloadRate)
}

func (s *shaperPlugin) onSessionUp(payload *l2tpevents.SessionUpPayload) {
	cfg := s.cfgPtr.Load()
	if cfg == nil {
		return
	}

	key := sessionKey{tunnelID: payload.TunnelID, sessionID: payload.SessionID}
	state := sessionState{
		iface:        payload.Interface,
		downloadRate: cfg.DefaultRate,
		uploadRate:   cfg.UploadRate,
		appliedAt:    time.Now(),
	}
	if state.uploadRate == 0 {
		state.uploadRate = cfg.DefaultRate
	}

	// RFC 2865 Section 5.11: Filter-Id from RADIUS overrides default rate.
	if meta := l2tp.LoadSessionMetadata(payload.TunnelID, payload.SessionID); meta != nil && meta.FilterID != "" {
		if down, up, ok := parseFilterRate(meta.FilterID); ok {
			state.downloadRate = down
			state.uploadRate = up
			logger().Info("l2tp-shaper: using RADIUS Filter-Id rate",
				"tunnel", payload.TunnelID, "session", payload.SessionID,
				"filter-id", meta.FilterID, "download-bps", down, "upload-bps", up)
		} else {
			logger().Debug("l2tp-shaper: Filter-Id present but not a rate; using default",
				"tunnel", payload.TunnelID, "session", payload.SessionID,
				"filter-id", meta.FilterID)
		}
	}

	if err := s.applyTC(payload.Interface, cfg.QdiscType, state.downloadRate); err != nil {
		logger().Warn("l2tp-shaper: failed to apply TC on session-up",
			"interface", payload.Interface, "error", err)
		return
	}

	s.sessions.Store(key, state)
	logger().Info("l2tp-shaper: applied shaping",
		"interface", payload.Interface,
		"tunnel", payload.TunnelID, "session", payload.SessionID,
		"rate-bps", state.downloadRate)
}

func (s *shaperPlugin) onSessionDown(payload *l2tpevents.SessionDownPayload) {
	key := sessionKey{tunnelID: payload.TunnelID, sessionID: payload.SessionID}
	val, loaded := s.sessions.LoadAndDelete(key)
	if !loaded {
		return
	}
	if state, ok := val.(sessionState); ok {
		if restorer, ok := traffic.GetBackend().(traffic.OriginalStateRestorer); ok {
			if err := restorer.RestoreOriginal(context.Background(), state.iface); err != nil {
				logger().Warn("l2tp-shaper: failed to restore original TC on session-down",
					"interface", state.iface, "error", err)
			}
		}
	}
	logger().Debug("l2tp-shaper: session removed from state",
		"tunnel", payload.TunnelID, "session", payload.SessionID)
}

func (s *shaperPlugin) onSessionRateChange(payload *l2tpevents.SessionRateChangePayload) {
	key := sessionKey{tunnelID: payload.TunnelID, sessionID: payload.SessionID}
	val, ok := s.sessions.Load(key)
	if !ok {
		logger().Warn("l2tp-shaper: rate-change for unknown session",
			"tunnel", payload.TunnelID, "session", payload.SessionID)
		return
	}
	state, ok2 := val.(sessionState)
	if !ok2 {
		return
	}

	cfg := s.cfgPtr.Load()
	qdiscType := traffic.QdiscTBF
	if cfg != nil {
		qdiscType = cfg.QdiscType
	}

	if err := s.applyTC(state.iface, qdiscType, payload.DownloadRate); err != nil {
		logger().Warn("l2tp-shaper: failed to update TC on rate-change",
			"interface", state.iface, "error", err)
		return
	}

	state.downloadRate = payload.DownloadRate
	state.uploadRate = payload.UploadRate
	state.appliedAt = time.Now()
	s.sessions.Store(key, state)

	logger().Info("l2tp-shaper: updated shaping",
		"interface", state.iface,
		"tunnel", payload.TunnelID, "session", payload.SessionID,
		"rate-bps", payload.DownloadRate)
}

func (s *shaperPlugin) applyTC(ifaceName string, qdiscType traffic.QdiscType, rateBps uint64) error {
	backend := traffic.GetBackend()
	if backend == nil {
		return errNoTrafficBackendLoadedConfigureTraffic
	}

	qos := traffic.InterfaceQoS{
		Interface: ifaceName,
		Qdisc: traffic.Qdisc{
			Type: qdiscType,
		},
	}

	if qdiscType == traffic.QdiscHTB {
		qos.Qdisc.DefaultClass = "default"
		qos.Qdisc.Classes = []traffic.TrafficClass{
			{
				Name: "default",
				Rate: rateBps,
				Ceil: rateBps,
			},
		}
	} else {
		qos.Qdisc.Classes = []traffic.TrafficClass{
			{
				Name: "default",
				Rate: rateBps,
			},
		}
	}

	desired := map[string]traffic.InterfaceQoS{ifaceName: qos}
	return backend.Apply(context.Background(), desired)
}

func (s *shaperPlugin) showSessions() any {
	type entry struct {
		TunnelID     uint16 `json:"tunnel-id"`
		SessionID    uint16 `json:"session-id"`
		Interface    string `json:"interface"`
		DownloadRate uint64 `json:"download-rate-bps"`
		UploadRate   uint64 `json:"upload-rate-bps"`
		AppliedAt    string `json:"applied-at"`
	}

	var entries []entry
	s.sessions.Range(func(key, val any) bool {
		k, ok := key.(sessionKey)
		if !ok {
			return true
		}
		st, ok := val.(sessionState)
		if !ok {
			return true
		}
		entries = append(entries, entry{
			TunnelID:     k.tunnelID,
			SessionID:    k.sessionID,
			Interface:    st.iface,
			DownloadRate: st.downloadRate,
			UploadRate:   st.uploadRate,
			AppliedAt:    st.appliedAt.UTC().Format(time.RFC3339),
		})
		return true
	})

	if entries == nil {
		entries = []entry{}
	}
	return entries
}
