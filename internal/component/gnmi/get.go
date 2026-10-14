// Design: docs/architecture/api/architecture.md -- gNMI Get RPC
// Related: server.go -- gNMI server core
// gNMI Specification Section 3.3: Get RPC

package gnmi

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// Get reads config state from the running tree and returns it as TypedValue.
func (s *Server) Get(_ context.Context, req *gpb.GetRequest) (*gpb.GetResponse, error) {
	if s.metrics != nil {
		s.metrics.requestsTotal.With("Get").Inc()
	}

	tree := s.tree()
	if tree == nil {
		s.recordError("Get", codes.Unavailable)
		return nil, status.Error(codes.Unavailable, "config tree not available")
	}

	if len(req.GetPath()) == 0 {
		s.recordError("Get", codes.InvalidArgument)
		return nil, status.Error(codes.InvalidArgument, "at least one path required")
	}

	schema, err := s.configSchema()
	if err != nil {
		s.recordError("Get", codes.Unavailable)
		return nil, status.Errorf(codes.Unavailable, "config schema not available: %v", err)
	}

	notifications := make([]*gpb.Notification, 0, len(req.GetPath()))
	ts := time.Now().UnixNano()

	for _, path := range req.GetPath() {
		segments, err := pathToSegments(schema, path)
		if err != nil {
			s.recordError("Get", codes.InvalidArgument)
			return nil, status.Errorf(codes.InvalidArgument, "path: %v", err)
		}
		if len(segments) == 0 {
			s.recordError("Get", codes.InvalidArgument)
			return nil, status.Error(codes.InvalidArgument, errEmptyPath.Error())
		}

		subtree, remaining := walkTree(tree, schema, segments)
		if subtree == nil || len(remaining) > 0 {
			s.recordError("Get", codes.NotFound)
			return nil, status.Errorf(codes.NotFound, "path not found: %s", pathString(segments))
		}

		val, err := treeToTypedValue(subtree, segments)
		if err != nil {
			s.recordError("Get", codes.Internal)
			return nil, status.Errorf(codes.Internal, "encode value: %v", err)
		}

		notifications = append(notifications, &gpb.Notification{
			Timestamp: ts,
			Update: []*gpb.Update{{
				// The keys are named from the schema, however the request named them.
				Path: segmentsToPath(schema, segments),
				Val:  val,
			}},
		})
	}

	return &gpb.GetResponse{Notification: notifications}, nil
}

// walkTree navigates the config tree following path segments.
// Returns the deepest tree node reached and any remaining segments. A list
// entry address spans the segments the config codec reads
// (config.ListPathStep); a list the schema does not hold takes one segment.
func walkTree(tree *zeconfig.Tree, schema *zeconfig.Schema, segments []string) (*zeconfig.Tree, []string) {
	current := tree
	var node zeconfig.ChildProvider
	if schema != nil {
		node = schema
	}
	for i := 0; i < len(segments); i++ {
		seg := segments[i]
		schemaChild := schemaNode(node, seg)
		node, _ = schemaChild.(zeconfig.ChildProvider)
		child := current.GetContainer(seg)
		if child != nil {
			current = child
			continue
		}
		list := current.GetList(seg)
		if list != nil {
			if i+1 >= len(segments) {
				return listToTree(list), nil
			}
			key, span := segments[i+1], 1
			if listNode, ok := schemaChild.(*zeconfig.ListNode); ok {
				addr, err := zeconfig.ListPathStep(seg, listNode, segments[i+1:])
				if err != nil {
					return nil, segments[i:]
				}
				key, span = addr.Key, addr.Tokens
			}
			if entry, ok := list[key]; ok {
				current = entry
				i += span
				continue
			}
		}
		if _, ok := current.Get(seg); ok && i == len(segments)-1 {
			return current, nil
		}
		return nil, segments[i:]
	}
	return current, nil
}

// treeToTypedValue encodes a tree node as a gNMI TypedValue.
// Leaf values are returned as StringVal; containers as JSON_IETF.
func treeToTypedValue(tree *zeconfig.Tree, segments []string) (*gpb.TypedValue, error) {
	if len(segments) > 0 {
		leafName := segments[len(segments)-1]
		if val, ok := tree.Get(leafName); ok {
			return &gpb.TypedValue{
				Value: &gpb.TypedValue_StringVal{StringVal: val},
			}, nil
		}
	}

	m := tree.ToMap()
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &gpb.TypedValue{
		Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: data},
	}, nil
}

// listToTree wraps a YANG list (map of entries) as a synthetic Tree
// so it can be serialized via ToMap for gNMI Get responses.
func listToTree(entries map[string]*zeconfig.Tree) *zeconfig.Tree {
	wrapper := zeconfig.NewTree()
	for key, entry := range entries {
		wrapper.SetContainer(key, entry)
	}
	return wrapper
}

func pathString(segments []string) string {
	n := len(segments)
	for _, s := range segments {
		n += len(s)
	}
	buf := make([]byte, 0, n)
	for _, s := range segments {
		buf = append(buf, '/')
		buf = append(buf, s...)
	}
	return string(buf)
}
