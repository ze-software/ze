// Design: docs/research/l2tpv2-ze-integration.md -- l2tp-shaper event handlers

package l2tpshaper

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

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
	if _, loaded := s.sessions.LoadAndDelete(key); loaded {
		logger().Debug("l2tp-shaper: session removed from state",
			"tunnel", payload.TunnelID, "session", payload.SessionID)
	}
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
		return fmt.Errorf("no traffic backend loaded; configure traffic-control or wait for it to start")
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

func (s *shaperPlugin) showSessions() string {
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
	b, err := json.Marshal(entries)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}
