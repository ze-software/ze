// Design: docs/contributing/github-integration.md -- one bounded credential-redacted Discord attempt.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type notificationTransportFixture func(*http.Request) (*http.Response, error)

func (transport notificationTransportFixture) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func notificationHTTPResponse(t *testing.T, code int, body string) *http.Response {
	t.Helper()
	response := &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	})
	return response
}

// TestNotificationDiscordConfirmsDestinationAndWaitsForMessage observes the
// production HTTP requests without sending to any network service.
func TestNotificationDiscordConfirmsDestinationAndWaitsForMessage(t *testing.T) {
	destination, err := newNotificationDestination("maintainers", "123", "456")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	sender, err := newDiscordNotificationSender("https://discord.com/api/webhooks/456/private-token", destination, notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.Host != "discord.com" {
			t.Error("request escaped the fixed origin")
		}
		if _, ok := request.Context().Deadline(); !ok {
			t.Error("request lacks an overall deadline")
		}
		if calls == 1 {
			if request.Method != http.MethodGet {
				t.Error("delivery did not check destination before POST")
			}
			return notificationHTTPResponse(t, 200, `{"id":"456","channel_id":"123"}`), nil
		}
		if calls != 2 {
			t.Error("sender retried a request")
		}
		if request.Method != http.MethodPost {
			t.Error("message was not a POST")
		}
		if request.URL.RawQuery != "wait=true" {
			t.Error("message did not request a confirmation")
		}
		if request.GetBody != nil {
			t.Error("POST body remains automatically replayable")
		}
		var body struct {
			Content  string `json:"content"`
			Mentions struct {
				Parse []string `json:"parse"`
			} `json:"allowed_mentions"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Content != "@everyone reviewed text" {
			t.Error("sender changed approved text")
		}
		if body.Mentions.Parse == nil {
			t.Error("mention policy was not explicitly supplied")
		}
		if len(body.Mentions.Parse) != 0 {
			t.Error("message can trigger mentions")
		}
		return notificationHTTPResponse(t, 200, `{"id":"900","channel_id":"123"}`), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	result, err := sender.Send(t.Context(), destination, "@everyone reviewed text")
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != notificationSendConfirmed {
		t.Errorf("outcome = %d", result.Outcome)
	}
	if result.MessageID != "900" {
		t.Error("message confirmation identity was lost")
	}
	if calls != 2 {
		t.Errorf("calls = %d, want identity GET and one POST", calls)
	}
}

// TestNotificationDiscordAmbiguityNeverRetriesOrLeaksCredentials covers
// transport loss, redirects, malformed acknowledgements and bounded responses.
func TestNotificationDiscordAmbiguityNeverRetriesOrLeaksCredentials(t *testing.T) {
	destination, err := newNotificationDestination("maintainers", "123", "456")
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name     string
		status   int
		body     string
		redirect string
		err      error
	}{
		{"lost acknowledgement", 0, "", "", errors.New("https://discord.com/api/webhooks/456/private-token")},
		{"redirect", http.StatusTemporaryRedirect, "", "https://example.com/steal", nil},
		{"server error", 500, "private-token", "", nil},
		{"malformed confirmation", 200, "{broken private-token", "", nil},
		{"malformed identity", 200, `{"id":"private-token","channel_id":"123"}`, "", nil},
		{"wrong channel", 200, `{"id":"900","channel_id":"124"}`, "", nil},
		{"oversized confirmation", 200, strings.Repeat(" ", 32*1024+1), "", nil},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			calls := 0
			sender, err := newDiscordNotificationSender("https://discord.com/api/webhooks/456/private-token", destination, notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return notificationHTTPResponse(t, 200, `{"id":"456","channel_id":"123"}`), nil
				}
				if fixture.err != nil {
					return nil, fixture.err
				}
				response := notificationHTTPResponse(t, fixture.status, fixture.body)
				if fixture.redirect != "" {
					response.Header.Set("Location", fixture.redirect)
				}
				return response, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			result, err := sender.Send(context.Background(), destination, "reviewed text")
			if err == nil {
				t.Fatal("uncertain delivery was confirmed")
			}
			if result.Outcome != notificationSendAmbiguous {
				t.Errorf("outcome = %d, want ambiguous", result.Outcome)
			}
			if calls != 2 {
				t.Errorf("calls = %d, want no redirected or repeated POST", calls)
			}
			if strings.Contains(err.Error(), "private-token") {
				t.Error("credential leaked into the public error")
			}
		})
	}
}

// TestNotificationDiscordRejectsMovedDestinationBeforePosting exercises the
// negative boundary of destination approval and refuses non-Discord origins.
func TestNotificationDiscordRejectsMovedDestinationBeforePosting(t *testing.T) {
	destination, err := newNotificationDestination("maintainers", "123", "456")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	transport := notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodGet {
			t.Error("moved webhook received a POST")
		}
		return notificationHTTPResponse(t, 200, `{"id":"456","channel_id":"124"}`), nil
	})
	sender, err := newDiscordNotificationSender("https://discord.com/api/webhooks/456/private-token", destination, transport)
	if err != nil {
		t.Fatal(err)
	}
	result, err := sender.Send(t.Context(), destination, "reviewed text")
	if err == nil {
		t.Error("moved webhook retained approval")
	}
	if result.Outcome != notificationSendRejected {
		t.Error("pre-POST rejection was misclassified")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want identity GET only", calls)
	}
	if _, err := newDiscordNotificationSender("https://example.com/api/webhooks/456/private-token", destination, transport); err == nil {
		t.Error("foreign webhook origin accepted")
	}
	if _, err := newDiscordNotificationSender("https://discord.com/api/webhooks/456/private-token?", destination, transport); err == nil {
		t.Error("empty query delimiter could hide the required wait parameter")
	}
}
