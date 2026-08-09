// Design: docs/architecture/l2tp/subscriber-session-model.md -- subscriber CLI handlers

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/show"

	_ "github.com/ze-software/ze/internal/component/l2tp/subscriber/cmd/yang"
)

var errRegistryUnavailable = errors.New("subscriber: registry not available")

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-subscriber-api:summary", Handler: handleSummary},
		pluginserver.RPCRegistration{WireMethod: "ze-subscriber-api:detail", Handler: handleDetail},
	)
}

func handleSummary(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	svc := subscriber.LookupService()
	if svc == nil {
		return errResponse(errRegistryUnavailable), nil
	}
	counts := svc.Registry.Count()
	sessions := svc.Registry.All()

	out := make([]map[string]any, 0, len(sessions))
	for i := range sessions {
		m := sessionBrief(&sessions[i])
		show.EnrichBrief("show subscriber", m)
		out = append(out, m)
	}

	payload := map[string]any{
		"total":    counts.Total,
		"pppoe":    counts.PPPoE,
		"l2tp":     counts.L2TP,
		"sessions": out,
	}
	return jsonResponse("subscriber summary", payload)
}

func handleDetail(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	svc := subscriber.LookupService()
	if svc == nil {
		return errResponse(errRegistryUnavailable), nil
	}
	id := ""
	if ctx != nil {
		id = ctx.Selector("id")
	}
	if id == "" && len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		return errResponse(errors.New("usage: show subscriber id <id> detail")), nil
	}
	sess, ok := svc.Registry.Get(id)
	if !ok {
		return errResponse(fmt.Errorf("subscriber: session %q not found", id)), nil
	}
	detail := sessionFull(&sess)
	show.Enrich("show subscriber detail", detail)
	return jsonResponse("subscriber detail", detail)
}

func sessionBrief(s *subscriber.Session) map[string]any {
	m := map[string]any{
		"id":          s.ID,
		"access-type": string(s.AccessType),
		"state":       string(s.State),
		"username":    s.Username,
		"interface":   s.PppInterface,
	}
	if s.IPv4Addr.IsValid() {
		m["ipv4"] = s.IPv4Addr.String()
	}
	if !s.ActivatedAt.IsZero() {
		m["duration"] = time.Since(s.ActivatedAt).Truncate(time.Second).String()
	}
	return m
}

func sessionFull(s *subscriber.Session) map[string]any {
	m := map[string]any{
		"id":          s.ID,
		"access-type": string(s.AccessType),
		"state":       string(s.State),
	}
	if s.Username != "" {
		m["username"] = s.Username
	}
	if s.AccessInterface != "" {
		m["access-interface"] = s.AccessInterface
	}
	if s.PppInterface != "" {
		m["ppp-interface"] = s.PppInterface
	}
	if s.NegotiatedMRU != 0 {
		m["negotiated-mru"] = s.NegotiatedMRU
	}
	if s.AuthMethod != "" {
		m["auth-method"] = s.AuthMethod
	}
	if s.PoolName != "" {
		m["pool"] = s.PoolName
	}
	if s.ServiceGroup != "" {
		m["service-group"] = s.ServiceGroup
	}
	if s.DownloadRate != 0 {
		m["download-rate"] = s.DownloadRate
	}
	if s.UploadRate != 0 {
		m["upload-rate"] = s.UploadRate
	}
	if s.AcctSessionID != "" {
		m["acct-session-id"] = s.AcctSessionID
	}
	if len(s.MAC) > 0 {
		m["mac"] = s.MAC.String()
	}
	if s.IPv4Addr.IsValid() {
		m["ipv4"] = s.IPv4Addr.String()
	}
	if s.DNSPrimary.IsValid() {
		m["dns-primary"] = s.DNSPrimary.String()
	}
	if s.DNSSecondary.IsValid() {
		m["dns-secondary"] = s.DNSSecondary.String()
	}
	if s.IPv6Prefix.IsValid() {
		m["ipv6-prefix"] = s.IPv6Prefix.String()
	}
	if !s.ActivatedAt.IsZero() {
		m["activated-at"] = s.ActivatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		m["duration"] = time.Since(s.ActivatedAt).Truncate(time.Second).String()
	}
	if s.PPPoESID != 0 {
		m["pppoe-sid"] = s.PPPoESID
	}
	if s.ServiceName != "" {
		m["service-name"] = s.ServiceName
	}
	if s.TunnelID != 0 || s.SessionID != 0 {
		m["tunnel-id"] = s.TunnelID
		m["session-id"] = s.SessionID
	}
	if s.PeerAddr.IsValid() {
		m["peer-addr"] = s.PeerAddr.String()
	}
	return m
}

func jsonResponse(_ string, payload any) (*plugin.Response, error) {
	if m, ok := payload.(map[string]any); ok {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(m)}, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON(data)}, nil
}

func errResponse(err error) *plugin.Response {
	return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}
}
