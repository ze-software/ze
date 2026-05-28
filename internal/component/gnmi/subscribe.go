// Design: docs/architecture/api/architecture.md -- gNMI Subscribe RPC
// Related: server.go -- gNMI server core
// gNMI Specification Section 3.5: Subscribe RPC

package gnmi

import (
	"encoding/json"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

const (
	maxSubscribeClients = 64
	subscribeSendBuffer = 32
)

// ChangeNotifier delivers config change events to subscribers.
type ChangeNotifier struct {
	mu      sync.RWMutex
	clients map[chan *gpb.Notification]struct{}
}

// NewChangeNotifier creates a notifier for broadcasting config changes.
func NewChangeNotifier() *ChangeNotifier {
	return &ChangeNotifier{
		clients: make(map[chan *gpb.Notification]struct{}),
	}
}

// Subscribe adds a client channel. Returns nil if the client limit is reached.
func (cn *ChangeNotifier) Subscribe() chan *gpb.Notification {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	if len(cn.clients) >= maxSubscribeClients {
		return nil
	}
	ch := make(chan *gpb.Notification, subscribeSendBuffer)
	cn.clients[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a client channel and closes it.
func (cn *ChangeNotifier) Unsubscribe(ch chan *gpb.Notification) {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	if _, ok := cn.clients[ch]; ok {
		delete(cn.clients, ch)
		close(ch)
	}
}

// Notify broadcasts a notification to all subscribed clients.
// Drops the event for clients whose buffer is full rather than blocking
// the notifier. This is a deliberate design choice: a slow consumer
// missing an event is better than blocking all config commits.
func (cn *ChangeNotifier) Notify(n *gpb.Notification) {
	cn.mu.RLock()
	defer cn.mu.RUnlock()
	for ch := range cn.clients {
		select {
		case ch <- n:
		default:
			// Buffer full: drop rather than block the commit path.
			logger.Warn("gNMI subscribe client buffer full, dropping notification")
		}
	}
}

// NotifyConfigReload broadcasts a generic config-changed notification to all
// subscribers. Used when config is committed from a non-gNMI source (web, CLI,
// managed) so STREAM subscribers can re-fetch current state.
func (cn *ChangeNotifier) NotifyConfigReload() {
	cn.Notify(&gpb.Notification{
		Timestamp: time.Now().UnixNano(),
		Update: []*gpb.Update{{
			Path: &gpb.Path{},
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "config-reload"}},
		}},
	})
}

// NotifyChange creates and broadcasts a change notification for a path.
func (cn *ChangeNotifier) NotifyChange(path []string, listNames map[string]bool, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		logger.Warn("gnmi: marshal change notification", "error", err)
		return
	}
	n := &gpb.Notification{
		Timestamp: time.Now().UnixNano(),
		Update: []*gpb.Update{{
			Path: segmentsToPath(path, listNames),
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: data},
			},
		}},
	}
	cn.Notify(n)
}

// Subscribe handles gNMI Subscribe RPCs (ONCE and STREAM modes).
func (s *Server) Subscribe(stream gpb.GNMI_SubscribeServer) error {
	if s.metrics != nil {
		s.metrics.requestsTotal.With("Subscribe").Inc()
	}

	req, err := stream.Recv()
	if err != nil {
		return err
	}

	sub := req.GetSubscribe()
	if sub == nil {
		s.recordError("Subscribe", codes.InvalidArgument)
		return status.Error(codes.InvalidArgument, "first message must be SubscribeRequest with subscribe field")
	}

	switch sub.GetMode() {
	case gpb.SubscriptionList_ONCE:
		return s.handleSubscribeOnce(stream, sub)
	case gpb.SubscriptionList_STREAM:
		return s.handleSubscribeStream(stream)
	case gpb.SubscriptionList_POLL:
		s.recordError("Subscribe", codes.Unimplemented)
		return status.Error(codes.Unimplemented, "POLL subscribe mode not supported")
	}

	s.recordError("Subscribe", codes.InvalidArgument)
	return status.Errorf(codes.InvalidArgument, "unknown subscribe mode %v", sub.GetMode())
}

func (s *Server) handleSubscribeOnce(stream gpb.GNMI_SubscribeServer, sub *gpb.SubscriptionList) error {
	tree := s.tree()
	if tree == nil {
		s.recordError("Subscribe", codes.Unavailable)
		return status.Error(codes.Unavailable, "config tree not available")
	}

	ts := time.Now().UnixNano()
	subs := sub.GetSubscription()

	if len(subs) == 0 {
		m := tree.ToMap()
		data, err := json.Marshal(m)
		if err != nil {
			s.recordError("Subscribe", codes.Internal)
			return status.Errorf(codes.Internal, "marshal config: %v", err)
		}
		if sendErr := stream.Send(&gpb.SubscribeResponse{
			Response: &gpb.SubscribeResponse_Update{Update: &gpb.Notification{
				Timestamp: ts,
				Update: []*gpb.Update{{
					Path: sub.GetPrefix(),
					Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: data}},
				}},
			}},
		}); sendErr != nil {
			return sendErr
		}
	} else {
		for _, subscription := range subs {
			path := subscription.GetPath()
			segments, pathErr := pathToSegments(path)
			if pathErr != nil {
				s.recordError("Subscribe", codes.InvalidArgument)
				return status.Errorf(codes.InvalidArgument, "subscribe path: %v", pathErr)
			}
			subtree, remaining := walkTree(tree, segments)
			if subtree == nil || len(remaining) > 0 {
				continue
			}
			val, encErr := treeToTypedValue(subtree, segments)
			if encErr != nil {
				s.recordError("Subscribe", codes.Internal)
				return status.Errorf(codes.Internal, "encode value: %v", encErr)
			}
			if sendErr := stream.Send(&gpb.SubscribeResponse{
				Response: &gpb.SubscribeResponse_Update{Update: &gpb.Notification{
					Timestamp: ts,
					Update:    []*gpb.Update{{Path: path, Val: val}},
				}},
			}); sendErr != nil {
				return sendErr
			}
		}
	}

	return stream.Send(&gpb.SubscribeResponse{
		Response: &gpb.SubscribeResponse_SyncResponse{SyncResponse: true},
	})
}

func (s *Server) handleSubscribeStream(stream gpb.GNMI_SubscribeServer) error {
	if s.notifier == nil {
		s.recordError("Subscribe", codes.Unavailable)
		return status.Error(codes.Unavailable, "change notifications not available")
	}

	ch := s.notifier.Subscribe()
	if ch == nil {
		s.recordError("Subscribe", codes.ResourceExhausted)
		return status.Error(codes.ResourceExhausted, "too many subscribe clients")
	}
	defer s.notifier.Unsubscribe(ch)

	if s.metrics != nil {
		s.metrics.subscribeActive.Inc()
		defer s.metrics.subscribeActive.Dec()
	}

	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case n, ok := <-ch:
			if !ok {
				return nil
			}
			if sendErr := stream.Send(&gpb.SubscribeResponse{
				Response: &gpb.SubscribeResponse_Update{Update: n},
			}); sendErr != nil {
				return sendErr
			}
		}
	}
}
