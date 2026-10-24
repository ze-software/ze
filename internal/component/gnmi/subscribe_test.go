package gnmi

import (
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestChangeNotifierSubscribeUnsubscribe(t *testing.T) {
	cn := NewChangeNotifier()
	ch := cn.Subscribe()
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	cn.Unsubscribe(ch)

	cn.mu.RLock()
	count := len(cn.clients)
	cn.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 clients after unsubscribe, got %d", count)
	}
}

func TestChangeNotifierMaxClients(t *testing.T) {
	cn := NewChangeNotifier()
	channels := make([]chan *gpb.Notification, 0, maxSubscribeClients)
	for range maxSubscribeClients {
		ch := cn.Subscribe()
		if ch == nil {
			t.Fatal("expected non-nil channel before limit")
		}
		channels = append(channels, ch)
	}

	overflow := cn.Subscribe()
	if overflow != nil {
		t.Fatal("expected nil channel at limit")
	}

	for _, ch := range channels {
		cn.Unsubscribe(ch)
	}
}

func TestChangeNotifierNotify(t *testing.T) {
	cn := NewChangeNotifier()
	ch := cn.Subscribe()
	defer cn.Unsubscribe(ch)

	n := &gpb.Notification{Timestamp: time.Now().UnixNano()}
	cn.Notify(n)

	select {
	case got := <-ch:
		if got.Timestamp != n.Timestamp {
			t.Errorf("timestamp mismatch: got %d, want %d", got.Timestamp, n.Timestamp)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestExternalCommitNotifiesGNMI(t *testing.T) {
	cn := NewChangeNotifier()
	ch := cn.Subscribe()
	defer cn.Unsubscribe(ch)

	cn.NotifyConfigReload()

	select {
	case got := <-ch:
		if got == nil {
			t.Fatal("expected non-nil notification")
		}
		if len(got.Update) != 1 {
			t.Fatalf("expected 1 update, got %d", len(got.Update))
		}
		val := got.Update[0].GetVal().GetStringVal()
		if val != "config-reload" {
			t.Errorf("expected config-reload value, got %q", val)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for config reload notification")
	}
}

func TestChangeNotifierNotifyChange(t *testing.T) {
	cn := NewChangeNotifier()
	ch := cn.Subscribe()
	defer cn.Unsubscribe(ch)

	cn.notifyChange([]string{"bgp", "router-id"}, nil, "1.2.3.4")

	select {
	case got := <-ch:
		if got == nil {
			t.Fatal("expected non-nil notification")
		}
		if len(got.Update) != 1 {
			t.Fatalf("expected 1 update, got %d", len(got.Update))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

// TestStreamNotificationAnswersEachJSONEncoding checks the STREAM path: the
// notifier shares one JSON_IETF notification between subscribers, and a JSON
// subscriber receives its values as JsonVal without the shared one changing.
// gNMI specification Section 2.3.1: "An implementation MUST NOT serialise data
// with mixed `JSON` and `JSON_IETF` encodings."
// Method: rewrap one notification for each encoding, then read the original.
func TestStreamNotificationAnswersEachJSONEncoding(t *testing.T) {
	cn := NewChangeNotifier()
	ch := cn.Subscribe()
	defer cn.Unsubscribe(ch)
	cn.notifyChange([]string{"bgp"}, nil, map[string]any{"router-id": "1.2.3.4"})
	var shared *gpb.Notification
	select {
	case shared = <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification")
	}
	want := string(shared.GetUpdate()[0].GetVal().GetJsonIetfVal())

	if got := answerEncodingJSONIETF.notification(shared); got != shared {
		t.Error("JSON_IETF subscriber did not get the shared notification")
	}
	json := answerEncodingJSON.notification(shared).GetUpdate()[0].GetVal()
	if string(json.GetJsonVal()) != want || json.GetJsonIetfVal() != nil {
		t.Errorf("JSON subscriber got %v, want JsonVal %s", json, want)
	}
	if string(shared.GetUpdate()[0].GetVal().GetJsonIetfVal()) != want {
		t.Errorf("the shared notification changed: %v", shared)
	}
}
