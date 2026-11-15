// Design: docs/contributing/github-integration.md -- a bounded webhook sender never retries an uncertain POST.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// discordNotificationSender contains the webhook credential privately. It is
// safe for concurrent use when its supplied transport is. Its zero value is
// invalid. The private activation boundary supplies its credential separately.
type discordNotificationSender struct {
	client      *http.Client
	endpoint    string
	destination notificationDestination
}

func newDiscordNotificationSender(raw string, destination notificationDestination, transport http.RoundTripper) (*discordNotificationSender, error) {
	if transport == nil {
		return nil, errors.New("discord notification transport is missing")
	}
	if err := notificationSnowflake(destination.channel); err != nil {
		return nil, fmt.Errorf("discord notification channel identity is invalid: %w", err)
	}
	if err := notificationSnowflake(destination.webhook); err != nil {
		return nil, fmt.Errorf("discord notification webhook identity is invalid: %w", err)
	}
	if len(raw) > 2048 {
		return nil, errors.New("discord webhook credential exceeds its bound")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("discord webhook credential is malformed")
	}
	if parsed.Scheme != "https" {
		return nil, errors.New("discord webhook must use HTTPS")
	}
	if parsed.Host != "discord.com" {
		return nil, errors.New("discord webhook must use the fixed Discord origin")
	}
	if parsed.User != nil {
		return nil, errors.New("discord webhook cannot contain URL user information")
	}
	if parsed.RawQuery != "" {
		return nil, errors.New("discord webhook cannot supply its own query parameters")
	}
	if parsed.ForceQuery {
		return nil, errors.New("discord webhook cannot contain an empty query delimiter")
	}
	if parsed.Fragment != "" {
		return nil, errors.New("discord webhook cannot contain a fragment")
	}
	if parsed.RawPath != "" {
		return nil, errors.New("discord webhook path must not use escaped aliases")
	}
	path, found := strings.CutPrefix(parsed.Path, "/api/webhooks/")
	if !found {
		path, found = strings.CutPrefix(parsed.Path, "/api/v10/webhooks/")
	}
	if !found {
		return nil, errors.New("discord webhook has an unsupported API path")
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return nil, errors.New("discord webhook has an unsupported path shape")
	}
	if parts[0] != destination.webhook {
		return nil, errors.New("discord webhook does not match the approved destination identity")
	}
	if parts[1] == "" {
		return nil, errors.New("discord webhook token is empty")
	}
	if len(parts[1]) > 256 {
		return nil, errors.New("discord webhook token exceeds its bound")
	}
	for _, r := range parts[1] {
		if notificationASCIIAlphanumeric(r) {
			continue
		}
		if strings.ContainsRune("._-", r) {
			continue
		}
		return nil, errors.New("discord webhook token has an unsupported character")
	}
	return &discordNotificationSender{destination: destination, endpoint: parsed.String(),
		client: &http.Client{Transport: transport, Timeout: 10 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// Send checks the webhook's current channel before its one POST, asks Discord
// to wait for confirmation and accepts only the returned message identity.
// Errors are deliberately generic: HTTP errors can carry the credential URL.
func (sender *discordNotificationSender) Send(ctx context.Context, destination notificationDestination, text string) (notificationSendResult, error) {
	rejected := notificationSendResult{Outcome: notificationSendRejected}
	ambiguous := notificationSendResult{Outcome: notificationSendAmbiguous}
	if sender == nil {
		return rejected, errors.New("discord sender was not constructed")
	}
	if sender.client == nil {
		return rejected, errors.New("discord sender has no HTTP configuration")
	}
	if destination != sender.destination {
		return rejected, errors.New("discord sender destination differs from the approved identity")
	}
	if text == "" {
		return rejected, errors.New("discord message is empty")
	}
	if len(text) > notificationTextMax {
		return rejected, errors.New("discord message exceeds its byte bound")
	}
	if !utf8.ValidString(text) {
		return rejected, errors.New("discord message is not valid UTF-8")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sender.endpoint, http.NoBody)
	if err != nil {
		return rejected, errors.New("discord webhook identity check could not be prepared")
	}
	response, err := sender.client.Do(request)
	if err != nil {
		return rejected, errors.New("discord webhook identity check failed before any message POST")
	}
	identity, err := notificationDiscordResponse(response)
	if err != nil {
		return rejected, errors.New("discord webhook identity could not be confirmed before any message POST")
	}
	if identity.ID != destination.webhook {
		return rejected, errors.New("discord webhook returned another identity before any message POST")
	}
	if identity.ChannelID != destination.channel {
		return rejected, errors.New("discord webhook moved from the approved channel; no message was posted")
	}
	body, err := json.Marshal(struct {
		Content         string `json:"content"`
		AllowedMentions struct {
			Parse []string `json:"parse"`
		} `json:"allowed_mentions"`
	}{Content: text, AllowedMentions: struct {
		Parse []string `json:"parse"`
	}{Parse: []string{}}})
	if err != nil {
		return rejected, errors.New("discord message could not be encoded")
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodPost, sender.endpoint+"?wait=true", bytes.NewReader(body))
	if err != nil {
		return rejected, errors.New("discord message POST could not be prepared")
	}
	request.GetBody = nil
	request.Header.Set("Content-Type", "application/json")
	response, err = sender.client.Do(request)
	if err != nil {
		return ambiguous, errors.New("discord message acknowledgement is unresolved; reconcile this attempt without retrying")
	}
	if response.StatusCode >= 400 {
		if response.StatusCode < 500 {
			if response.StatusCode != http.StatusRequestTimeout {
				closeErr := response.Body.Close()
				if closeErr != nil {
					return rejected, errors.New("discord refused the POST and its response could not be closed")
				}
				return rejected, errors.New("discord refused the POST; no automatic retry is permitted")
			}
		}
	}
	identity, err = notificationDiscordResponse(response)
	if err != nil {
		return ambiguous, errors.New("discord message confirmation is missing or malformed; reconcile this attempt without retrying")
	}
	if identity.ChannelID != destination.channel {
		return ambiguous, errors.New("discord message confirmation names another channel; reconcile the attempted delivery")
	}
	if err := notificationSnowflake(identity.ID); err != nil {
		return ambiguous, fmt.Errorf("discord message confirmation identity is invalid: %w", err)
	}
	return notificationSendResult{Outcome: notificationSendConfirmed, MessageID: identity.ID}, nil
}

type notificationDiscordIdentity struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
}

func notificationDiscordResponse(response *http.Response) (notificationDiscordIdentity, error) {
	defer response.Body.Close() //nolint:errcheck // The bounded body read and confirmation identity decide the outcome.
	if response.StatusCode != http.StatusOK {
		return notificationDiscordIdentity{}, errors.New("discord response did not confirm success")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 32*1024+1))
	if err != nil {
		return notificationDiscordIdentity{}, errors.New("discord confirmation body could not be read")
	}
	if len(body) > 32*1024 {
		return notificationDiscordIdentity{}, errors.New("discord confirmation body exceeded its bound")
	}
	var identity notificationDiscordIdentity
	if err := json.Unmarshal(body, &identity); err != nil {
		return identity, errors.New("discord confirmation body was malformed")
	}
	return identity, nil
}
