// Design: docs/architecture/api/process-protocol.md -- original-connection scope echoes.
package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Both raw callback contracts must reject a valid reply for another incarnation.
// A second exact reply on the same real pipe remains useful after that refusal.
// MUTATION: bypassing raw reply validation accepts the other owner's snapshot.
func TestRawSnapshotReadKeepsExactConnectionScope(t *testing.T) {
	t.Parallel()
	for _, method := range []string{rpc.MethodCurrentSelection, rpc.MethodOriginAdmissionLookup} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			client, provider := newTestPluginConn(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			// One fixture lifecycle serves the two bounded wire exchanges.
			go func() {
				for index := range 2 {
					request, err := provider.ReadRequest(ctx)
					if err != nil {
						done <- err
						return
					}
					if request.Method != method {
						done <- fmt.Errorf("callback method = %q, want %q", request.Method, method)
						return
					}
					output, err := rawSnapshotScopeReply(request, index == 0)
					if err != nil {
						done <- err
						return
					}
					if err := provider.SendResult(ctx, request.ID, output); err != nil {
						done <- err
						return
					}
				}
				done <- nil
			}()
			joined := false
			t.Cleanup(func() {
				cancel()
				client.Close()   //nolint:errcheck // Cleanup ends this owned test pipe before joining its reader.
				provider.Close() //nolint:errcheck // Cleanup ends this owned test pipe before joining its reader.
				if joined {
					return
				}
				cleanup, finish := context.WithTimeout(context.Background(), time.Second)
				defer finish()
				select {
				case <-done: // Early assertion failure intentionally closes the owned pipe.
				case <-cleanup.Done():
					t.Error("raw snapshot provider did not join")
				}
			})
			output, err := rawSnapshotScopeRead(ctx, client, method)
			require.Error(t, err, "another owner's otherwise valid snapshot must be refused")
			require.Nil(t, output)
			output, err = rawSnapshotScopeRead(ctx, client, method)
			require.NoError(t, err)
			require.NotNil(t, output, "the exact original scope remains useful on the same pipe")
			select {
			case err := <-done:
				joined = true
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("raw snapshot provider did not finish both exchanges")
			}
		})
	}
}

func rawSnapshotScopeRead(ctx context.Context, client *PluginConn, method string) (any, error) {
	switch method {
	case rpc.MethodCurrentSelection:
		return client.SendCurrentSelection(ctx, rpc.CurrentSelectionInput{Destination: "192.0.2.80", DestinationSession: 71,
			Keys: []rpc.CurrentSelectionKey{{Family: "ipv4/unicast", KeyNLRI: []byte{24, 198, 51, 100}, Revision: 3}}})
	case rpc.MethodOriginAdmissionLookup:
		return client.SendOriginAdmissionLookup(ctx, rpc.OriginAdmissionLookupInput{AdmissionID: 7, Family: "ipv4/unicast",
			Prefix: "198.51.100.0/24", Destination: "192.0.2.80", PeerOwner: 71, Operation: rpc.OriginIntakeAnnounce})
	}
	return nil, fmt.Errorf("unknown raw snapshot fixture method %q", method)
}

func rawSnapshotScopeReply(request *rpc.Request, wrongOwner bool) (any, error) {
	switch request.Method {
	case rpc.MethodCurrentSelection:
		var input rpc.CurrentSelectionInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		output := &rpc.CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession, Cut: 3}
		for _, key := range input.Keys {
			output.Results = append(output.Results, rpc.CurrentSelectionResult{Family: key.Family, KeyNLRI: key.KeyNLRI,
				Revision: key.Revision, Presence: rpc.CurrentSelectionAbsent})
		}
		if wrongOwner {
			output.DestinationSession++
		}
		return output, nil
	case rpc.MethodOriginAdmissionLookup:
		var input rpc.OriginAdmissionLookupInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		output := &rpc.OriginAdmissionLookupOutput{AdmissionID: input.AdmissionID, Family: input.Family, Prefix: input.Prefix,
			Destination: input.Destination, PeerOwner: input.PeerOwner, Operation: input.Operation, State: rpc.OriginAdmissionCurrent,
			Snapshot: rpc.OriginAdmissionSnapshot{Protocol: "static", NextHop: "192.0.2.1", PeerScope: rpc.OriginAdmissionAllPeers}}
		if wrongOwner {
			output.PeerOwner++
		}
		return output, nil
	}
	return nil, fmt.Errorf("unknown raw snapshot fixture method %q", request.Method)
}
