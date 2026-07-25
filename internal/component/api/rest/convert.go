// Design: docs/architecture/api/architecture.md -- REST/domain conversion helpers

package rest

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/api"
)

func fromRESTExecuteRequest(caller api.CallerIdentity, command string, params map[string]any) (*api.ExecuteRequest, error) {
	stringParams := make(map[string]string, len(params))
	for key, val := range params {
		var sval string
		if s, ok := val.(string); ok {
			sval = s
		} else {
			sval = fmt.Sprint(val)
		}
		if sval != "" {
			stringParams[key] = sval
		}
	}
	built, err := api.BuildCommand(command, stringParams)
	if err != nil {
		return nil, err
	}
	return &api.ExecuteRequest{Caller: caller, Command: built}, nil
}

func fromRESTStreamRequest(caller api.CallerIdentity, command string) *api.StreamRequest {
	return &api.StreamRequest{Caller: caller, Command: command}
}

func fromRESTListCommandsRequest(prefix string) *api.ListCommandsRequest {
	return &api.ListCommandsRequest{Prefix: prefix}
}

func fromRESTDescribeCommandRequest(path string) *api.DescribeCommandRequest {
	return &api.DescribeCommandRequest{Path: path}
}

func fromRESTConfigSetRequest(username, sessionID, path, value string) *api.ConfigSetRequest {
	return &api.ConfigSetRequest{
		Username:  username,
		SessionID: sessionID,
		Path:      path,
		Value:     value,
	}
}

func fromRESTConfigDeleteRequest(username, sessionID, path string) *api.ConfigDeleteRequest {
	return &api.ConfigDeleteRequest{
		Username:  username,
		SessionID: sessionID,
		Path:      path,
	}
}

func fromRESTConfigDiffRequest(username, sessionID string) *api.ConfigDiffRequest {
	return &api.ConfigDiffRequest{
		Username:  username,
		SessionID: sessionID,
	}
}

func fromRESTConfigCommitRequest(username, sessionID string) *api.ConfigCommitRequest {
	return &api.ConfigCommitRequest{
		Username:  username,
		SessionID: sessionID,
	}
}

func fromRESTConfigDiscardRequest(username, sessionID string) *api.ConfigDiscardRequest {
	return &api.ConfigDiscardRequest{
		Username:  username,
		SessionID: sessionID,
	}
}
