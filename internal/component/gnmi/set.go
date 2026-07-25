// Design: docs/architecture/api/architecture.md -- gNMI Set RPC
// Related: server.go -- gNMI server core
// gNMI Specification Section 3.4: Set RPC

package gnmi

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const gnmiUsername = "gnmi"

// Set applies config modifications atomically via a ConfigSessionManager.
// All updates, replaces, and deletes in a single SetRequest are applied
// within one config session and committed together.
func (s *Server) Set(ctx context.Context, req *gpb.SetRequest) (*gpb.SetResponse, error) {
	if s.metrics != nil {
		s.metrics.requestsTotal.With("Set").Inc()
	}

	if s.sessions == nil {
		s.recordError("Set", codes.Unavailable)
		return nil, status.Error(codes.Unavailable, "config sessions not available")
	}

	total := len(req.GetDelete()) + len(req.GetReplace()) + len(req.GetUpdate())
	if total == 0 {
		s.recordError("Set", codes.InvalidArgument)
		return nil, status.Error(codes.InvalidArgument, "no operations in set request")
	}

	sessionID, err := s.sessions.Enter(gnmiUsername)
	if err != nil {
		s.recordError("Set", codes.Internal)
		return nil, status.Errorf(codes.Internal, "create config session: %v", err)
	}

	results := make([]*gpb.UpdateResult, 0, total)

	for _, del := range req.GetDelete() {
		segments, pathErr := pathToSegments(del)
		if pathErr != nil {
			s.recordError("Set", codes.InvalidArgument)
			s.discardSession(ctx, sessionID)
			return nil, status.Errorf(codes.InvalidArgument, "delete path: %v", pathErr)
		}
		if len(segments) == 0 {
			s.recordError("Set", codes.InvalidArgument)
			s.discardSession(ctx, sessionID)
			return nil, status.Error(codes.InvalidArgument, "empty delete path")
		}
		deleteErr := s.sessions.DeleteSegments(gnmiUsername, sessionID, segments)
		if deleteErr != nil {
			s.recordError("Set", codes.InvalidArgument)
			s.discardSession(ctx, sessionID)
			return nil, status.Errorf(codes.InvalidArgument, "delete %s: %v", pathString(segments), deleteErr)
		}
		results = append(results, &gpb.UpdateResult{
			Path: del,
			Op:   gpb.UpdateResult_DELETE,
		})
	}

	for _, replace := range req.GetReplace() {
		if replaceErr := s.applyUpdate(sessionID, replace); replaceErr != nil {
			s.recordError("Set", codes.InvalidArgument)
			s.discardSession(ctx, sessionID)
			return nil, replaceErr
		}
		results = append(results, &gpb.UpdateResult{
			Path: replace.GetPath(),
			Op:   gpb.UpdateResult_REPLACE,
		})
	}

	for _, update := range req.GetUpdate() {
		if updateErr := s.applyUpdate(sessionID, update); updateErr != nil {
			s.recordError("Set", codes.InvalidArgument)
			s.discardSession(ctx, sessionID)
			return nil, updateErr
		}
		results = append(results, &gpb.UpdateResult{
			Path: update.GetPath(),
			Op:   gpb.UpdateResult_UPDATE,
		})
	}

	commitErr := s.sessions.Commit(&api.ConfigCommitRequest{
		Username:  gnmiUsername,
		SessionID: sessionID,
	})
	if commitErr != nil {
		s.recordError("Set", codes.Aborted)
		s.discardSession(ctx, sessionID)
		return nil, status.Errorf(codes.Aborted, "commit: %v", commitErr)
	}

	ts := time.Now().UnixNano()
	if s.notifier != nil {
		s.notifySetResults(req, ts)
	}

	return &gpb.SetResponse{
		Response:  results,
		Timestamp: ts,
	}, nil
}

// applyUpdate sets a single path/value via the config session.
func (s *Server) applyUpdate(sessionID string, update *gpb.Update) error {
	segments, err := pathToSegments(update.GetPath())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "update path: %v", err)
	}
	if len(segments) == 0 {
		return status.Error(codes.InvalidArgument, "empty update path")
	}

	value, err := typedValueToString(update.GetVal())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "update value: %v", err)
	}

	return s.sessions.SetSegments(gnmiUsername, sessionID, segments, value)
}

// typedValueToString extracts a string value from a gNMI TypedValue.
func typedValueToString(tv *gpb.TypedValue) (string, error) {
	if tv == nil {
		return "", nil
	}
	switch v := tv.Value.(type) {
	case *gpb.TypedValue_StringVal:
		return v.StringVal, nil
	case *gpb.TypedValue_IntVal:
		return textbuf.StringInt(v.IntVal), nil
	case *gpb.TypedValue_UintVal:
		return textbuf.StringUint(v.UintVal), nil
	case *gpb.TypedValue_BoolVal:
		if v.BoolVal {
			return "true", nil
		}
		return "false", nil
	case *gpb.TypedValue_JsonIetfVal:
		return string(v.JsonIetfVal), nil
	case *gpb.TypedValue_JsonVal:
		return string(v.JsonVal), nil
	default:
		return "", status.Error(codes.Unimplemented, "unsupported value encoding")
	}
}

func (s *Server) discardSession(_ context.Context, sessionID string) {
	_ = s.sessions.Discard(&api.ConfigDiscardRequest{
		Username:  gnmiUsername,
		SessionID: sessionID,
	})
}

// notifySetResults broadcasts change notifications for all paths modified by a Set.
func (s *Server) notifySetResults(req *gpb.SetRequest, ts int64) {
	for _, del := range req.GetDelete() {
		s.notifier.Notify(&gpb.Notification{
			Timestamp: ts,
			Delete:    []*gpb.Path{del},
		})
	}
	for _, upd := range req.GetReplace() {
		s.notifier.Notify(&gpb.Notification{
			Timestamp: ts,
			Update:    []*gpb.Update{{Path: upd.GetPath(), Val: upd.GetVal()}},
		})
	}
	for _, upd := range req.GetUpdate() {
		s.notifier.Notify(&gpb.Notification{
			Timestamp: ts,
			Update:    []*gpb.Update{{Path: upd.GetPath(), Val: upd.GetVal()}},
		})
	}
}
