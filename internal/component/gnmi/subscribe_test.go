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

	listNames := map[string]bool{"neighbor": true}
	cn.NotifyChange([]string{"bgp", "router-id"}, listNames, "1.2.3.4")

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
