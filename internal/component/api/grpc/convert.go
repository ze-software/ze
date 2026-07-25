// Design: docs/architecture/api/architecture.md -- proto/domain conversion helpers

package grpc

import (
	"encoding/json"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/internal/component/api"
)

func fromProtoExecuteRequest(pb *zepb.CommandRequest, caller api.CallerIdentity) (*api.ExecuteRequest, error) {
	command, err := api.BuildCommand(pb.GetCommand(), pb.GetParams())
	if err != nil {
		return nil, err
	}
	return &api.ExecuteRequest{Caller: caller, Command: command}, nil
}

func fromProtoStreamRequest(pb *zepb.CommandRequest, caller api.CallerIdentity) (*api.StreamRequest, error) {
	command, err := api.BuildCommand(pb.GetCommand(), pb.GetParams())
	if err != nil {
		return nil, err
	}
	return &api.StreamRequest{Caller: caller, Command: command}, nil
}

func fromProtoListCommandsRequest(pb *zepb.ListCommandsRequest) *api.ListCommandsRequest {
	return &api.ListCommandsRequest{Prefix: pb.GetPrefix()}
}

func fromProtoDescribeCommandRequest(pb *zepb.DescribeCommandRequest) *api.DescribeCommandRequest {
	return &api.DescribeCommandRequest{Path: pb.GetPath()}
}

func fromProtoConfigSetRequest(pb *zepb.ConfigSetRequest, username string) *api.ConfigSetRequest {
	return &api.ConfigSetRequest{
		Username:  username,
		SessionID: pb.GetSessionId(),
		Path:      pb.GetPath(),
		Value:     pb.GetValue(),
	}
}

func fromProtoConfigDeleteRequest(pb *zepb.ConfigDeleteRequest, username string) *api.ConfigDeleteRequest {
	return &api.ConfigDeleteRequest{
		Username:  username,
		SessionID: pb.GetSessionId(),
		Path:      pb.GetPath(),
	}
}

func fromProtoSessionRequest(pb *zepb.SessionRequest, username string) *api.ConfigDiffRequest {
	return &api.ConfigDiffRequest{
		Username:  username,
		SessionID: pb.GetSessionId(),
	}
}

func fromProtoCommitRequest(pb *zepb.CommitRequest, username string) *api.ConfigCommitRequest {
	return &api.ConfigCommitRequest{
		Username:  username,
		SessionID: pb.GetSessionId(),
	}
}

func fromProtoDiscardRequest(pb *zepb.SessionRequest, username string) *api.ConfigDiscardRequest {
	return &api.ConfigDiscardRequest{
		Username:  username,
		SessionID: pb.GetSessionId(),
	}
}

func execResultToProto(r *api.ExecResult) *zepb.CommandResponse {
	if r == nil {
		return &zepb.CommandResponse{Status: api.StatusError, Error: "nil result"}
	}
	resp := &zepb.CommandResponse{
		Status: r.Status,
		Error:  r.Error,
	}
	if r.Data != nil {
		data, err := json.Marshal(r.Data)
		if err == nil {
			resp.Data = data
		}
	}
	return resp
}

func commandMetaToProto(cmd api.CommandMeta) *zepb.CommandInfo {
	info := &zepb.CommandInfo{
		Name:        cmd.Name,
		Description: cmd.Description,
		ReadOnly:    cmd.ReadOnly,
	}
	if len(cmd.Params) > 0 {
		info.Params = make([]*zepb.ParamInfo, len(cmd.Params))
		for i, p := range cmd.Params {
			info.Params[i] = &zepb.ParamInfo{
				Name:        p.Name,
				Type:        p.Type,
				Description: p.Description,
				Required:    p.Required,
			}
		}
	}
	return info
}
