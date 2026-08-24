// Design: docs/architecture/show-enricher.md -- CoS enricher for subscriber show commands
//
// The build constraint is session_state.go's: these enrichers read
// sessionStore, which only the ze_l2tp handler fills. `show subscriber` is a
// BNG command, so a build without ze_l2tp has neither the command nor a
// session to enrich.

//go:build ze_l2tp

package cos

import (
	"sort"

	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/show"
)

func init() {
	show.MustRegister("show subscriber detail", "cos", show.Enricher{
		Detail: enrichSubscriberDetail,
	})
	show.MustRegister("show subscriber", "cos", show.Enricher{
		Brief: enrichSubscriberBrief,
	})
}

func enrichSubscriberDetail(base map[string]any) {
	tid, sid, ok := extractSessionKey(base)
	if !ok {
		return
	}
	v, loaded := sessionStore.Load(sessionKey{tunnelID: tid, sessionID: sid})
	if !loaded {
		return
	}
	state, ok := v.(sessionCoSState)
	if !ok {
		return
	}

	cosData := map[string]any{
		"profile": state.profileName,
	}

	if profile, found := coreCos.Lookup(state.profileName); found {
		cosData["ingress"] = qosMapToSlice(profile.IngressMap)
		cosData["egress"] = qosMapToSlice(profile.EgressMap)
	}

	base["cos"] = cosData
}

func enrichSubscriberBrief(base map[string]any) {
	tid, sid, ok := extractSessionKey(base)
	if !ok {
		return
	}
	v, loaded := sessionStore.Load(sessionKey{tunnelID: tid, sessionID: sid})
	if !loaded {
		return
	}
	state, ok := v.(sessionCoSState)
	if !ok {
		return
	}
	base["cos-profile"] = state.profileName
}

func extractSessionKey(base map[string]any) (uint16, uint16, bool) {
	tid, ok1 := base["tunnel-id"].(uint16)
	sid, ok2 := base["session-id"].(uint16)
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	return tid, sid, true
}

type qosEntry struct {
	From uint32 `json:"from"`
	To   uint32 `json:"to"`
}

func qosMapToSlice(m map[uint32]uint32) []qosEntry {
	entries := make([]qosEntry, 0, len(m))
	for from, to := range m {
		entries = append(entries, qosEntry{From: from, To: to})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].From < entries[j].From })
	return entries
}
