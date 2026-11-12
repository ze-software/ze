// Design: docs/guide/l2tp.md -- confirmed, atomic dynamic authorization
// Related: coa.go -- authenticated CoA admission and response

package l2tpauthradius

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
)

type coaCommandDispatch func(context.Context, string, []string, string) (string, json.RawMessage, error)

type coaOwnerRequest struct {
	Operation    string `json:"operation"`
	Token        string `json:"token,omitempty"`
	TunnelID     uint16 `json:"tunnel-id,omitempty"`
	SessionID    uint16 `json:"session-id,omitempty"`
	Interface    string `json:"interface,omitempty"`
	DownloadRate uint64 `json:"download-rate,omitempty"`
	UploadRate   uint64 `json:"upload-rate,omitempty"`
	Profile      string `json:"profile,omitempty"`
}

type coaOwner struct {
	command string
	token   string
}

// confirmCoA waits for the rate and CoS owners' synchronous backend results.
// It uses the existing command dispatcher, whose DirectBridge and RPC routes
// both authorize the calling plugin and preserve the source peer. No pointer or
// callback crosses a plugin boundary.
// RFC 5176 Section 2.3: "If the CoA-Request is unsuccessful for any matching
// sessions, the NAS MUST send a CoA-NAK in reply, and the requested authorization
// changes MUST NOT be made for any of the matching sessions."
func (cl *coaListener) confirmCoA(sub *subscriber.Session, change *coaChange, peer string) (failure error) {
	defer func() { recordCoAHealth(sub, failure) }()
	if cl.cfg.Dispatch == nil {
		return errors.New("no command dispatcher is available to confirm authorization changes")
	}
	var owners []coaOwner
	prepare := func(command string, request coaOwnerRequest) error {
		request.Operation = "prepare"
		data, err := cl.callCoAOwner(command, request, peer)
		if err != nil {
			return err
		}
		var result struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}
		if result.Token == "" {
			return errors.New("authorization owner returned no prepared transaction token")
		}
		owners = append(owners, coaOwner{command: command, token: result.Token})
		return nil
	}
	rollback := func(cause error) error {
		errs := []error{cause}
		// The owner set is bounded by the two authorization controls above.
		for i := len(owners) - 1; i >= 0; i-- { //nolint:modernize // the producer-sha of three RFC 5176 discrimination records covers confirmCoA's code; rewriting this loop stales them
			_, err := cl.callCoAOwner(owners[i].command, coaOwnerRequest{Operation: "rollback", Token: owners[i].token}, peer)
			if err != nil {
				errs = append(errs, fmt.Errorf("restore authorization owner %q: %w", owners[i].command, err))
			}
		}
		return errors.Join(errs...)
	}
	if change.hasRate {
		if err := prepare("request l2tp shaper rate", coaOwnerRequest{
			TunnelID: sub.TunnelID, SessionID: sub.SessionID, Interface: sub.PppInterface,
			DownloadRate: change.download, UploadRate: change.upload,
		}); err != nil {
			return rollback(err)
		}
	}
	if change.cosProfile != "" {
		if err := prepare("request class-of-service session", coaOwnerRequest{
			TunnelID: sub.TunnelID, SessionID: sub.SessionID, Interface: sub.AccessInterface, Profile: change.cosProfile,
		}); err != nil {
			return rollback(err)
		}
	}
	for _, operation := range []string{"apply", "commit"} {
		for _, owner := range owners {
			if _, err := cl.callCoAOwner(owner.command, coaOwnerRequest{Operation: operation, Token: owner.token}, peer); err != nil {
				return rollback(err)
			}
		}
	}
	return nil
}

func (cl *coaListener) callCoAOwner(command string, request coaOwnerRequest, peer string) (json.RawMessage, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	// Every operation gets a fresh bound, including compensation after a timed
	// out apply. SDK callback serialization puts that compensation after apply.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status, result, err := cl.cfg.Dispatch(ctx, command, []string{"transaction", string(data)}, peer)
	if err != nil {
		return nil, err
	}
	if status != "done" {
		return nil, fmt.Errorf("authorization owner %q returned status %q", command, status)
	}
	return result, nil
}
