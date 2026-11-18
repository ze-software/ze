// Design: docs/architecture/api/process-protocol.md -- batch validation reaches its actual receive-store owner.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// dispatchBatchValidation retains one context and one complete command. The
// dispatcher captures its exact RegisteredCommand.Process and connection; a
// default plugin name or process-global callback cannot choose another store.
func (s *Server) dispatchBatchValidation(ctx context.Context, proc *process.Process, decisions []rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.dispatcher == nil {
		return nil, opererr.New("RPKI validation has no registered received-route store",
			"Pending routes cannot be validated; configure a receive-store process serving "+rpc.BatchValidateCommand+" before restarting Ze", "ze request restart")
	}
	args := make([]string, 0, len(decisions)*9)
	for i, decision := range decisions {
		if decision.Accept && decision.Ineligible {
			return nil, opererr.New("RPKI validation decision "+strconv.Itoa(i)+" both accepts and excludes the same route",
				"No decision in this batch was sent; the validator must choose exactly one action", "ze show errors")
		}
		action := "r"
		if decision.Accept {
			action = "a"
		} else if decision.Ineligible {
			action = "i"
		}
		args = append(args, action, decision.PeerAddr, decision.Family, decision.Prefix,
			strconv.FormatUint(uint64(decision.PathID), 10), strconv.FormatUint(uint64(decision.ValState), 10),
			strconv.FormatUint(decision.MsgID, 10), strconv.FormatUint(decision.PeerOwner, 10), strconv.FormatUint(decision.ConnectionID, 10))
	}
	out, err := s.dispatchCommandArgs(ctx, proc, rpc.BatchValidateCommand, args, "")
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, opererr.New("The received-route store answered no RPKI validation result",
			"The whole batch remains unconfirmed; inspect the receive-store error before retrying", "ze show errors")
	}
	if out.Status != rpc.StatusDone || out.Error != "" {
		return nil, opererr.Wrap(errors.New(out.Error), "The received-route store refused the RPKI validation batch",
			"A partial or failed command cannot acknowledge validation of these routes", "ze show errors")
	}
	var result *rpc.BatchValidateResult
	if err := json.Unmarshal(out.Data, &result); err != nil {
		return nil, opererr.Wrap(err, "The received-route store returned an unreadable RPKI validation result",
			"The whole batch remains unconfirmed; inspect the receive-store error before retrying", "ze show errors")
	}
	if result == nil {
		return nil, opererr.New("The received-route store answered no RPKI validation result",
			"A null result cannot acknowledge validation of these routes", "ze show errors")
	}
	return result, nil
}
