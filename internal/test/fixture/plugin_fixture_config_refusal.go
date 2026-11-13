// Design: docs/architecture/api/architecture.md -- gRPC and gNMI share the commit hook.

package fixture

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRefusalFix = "fix: set bgp peer peer1 session capability route-refresh require"
const configKeyRefusalFix = "fix: set ospf redistribute <connected|static|kernel|bgp|isis> nssa-propagate true"

func configRefusalClient(address, certificate string) (*grpc.ClientConn, error) {
	cert, err := os.ReadFile(certificate)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(cert) {
		return nil, fmt.Errorf("TLS trust file %q holds no certificate", certificate)
	}
	return grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
	})))
}

func configEnumStatus(err error, code codes.Code) error {
	if status.Code(err) != code {
		return fmt.Errorf("enum refusal code=%s, want %s: %w", status.Code(err), code, err)
	}
	text := status.Convert(err).Message()
	if err := plugin01RefusalHolds("enum commit", text, []string{
		`bgp peer peer1 session capability route-refresh: "requre" is not an accepted value (did you mean require?)`,
		"route-refresh accepts only these values:",
		"  enable", "  disable", "  require", "  refuse", configRefusalFix,
	}, []string{"line ", "route-refresh require;", "invalid enum"}); err != nil {
		return err
	}
	if !strings.HasSuffix(strings.TrimSpace(text), configRefusalFix) {
		return fmt.Errorf("enum refusal does not end with the set fix: %q", text)
	}
	return nil
}

func configKeyStatus(err error) error {
	if status.Code(err) != codes.InvalidArgument {
		return fmt.Errorf("key refusal code=%s, want InvalidArgument: %w", status.Code(err), err)
	}
	text := status.Convert(err).Message()
	if err := plugin01RefusalHolds("enum key", text, []string{
		`ospf redistribute bogus: "bogus" is not an accepted value`,
		"source accepts only these values:",
		"  connected", "  static", "  kernel", "  bgp", "  isis", configKeyRefusalFix,
	}, []string{"line ", "invalid key for"}); err != nil {
		return err
	}
	if !strings.HasSuffix(strings.TrimSpace(text), configKeyRefusalFix) {
		return fmt.Errorf("key refusal does not end with the complete set command: %q", text)
	}
	return nil
}

func grpcConfigEnumRefusal(ctx context.Context, address, certificate string) error {
	conn, err := configRefusalClient(address, certificate)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // Closing the client changes no completed assertion.
	ctx, cancel := context.WithTimeout(ctx, WaitBudget(70, 20*time.Second))
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer secret")
	client := zepb.NewZeConfigServiceClient(conn)
	var before *zepb.ConfigResponse
	if !Poll(ctx, WaitAttempts(40, plugin01PollDelay, 40), plugin01PollDelay, func() bool {
		before, err = client.GetRunningConfig(ctx, &zepb.Empty{})
		return err == nil
	}) {
		return fmt.Errorf("TLS gRPC config listener did not become ready: %w", err)
	}
	created, err := client.EnterSession(ctx, &zepb.Empty{})
	if err != nil {
		return err
	}
	if created.GetSessionId() == "" {
		return fmt.Errorf("gRPC EnterSession returned no session ID")
	}
	_, err = client.SetConfig(ctx, &zepb.ConfigSetRequest{
		SessionId: created.GetSessionId(),
		Path:      "ospf.redistribute.bogus.nssa-propagate",
		Value:     "true",
	})
	if err := configKeyStatus(err); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: gRPC enum key refused with InvalidArgument and the complete set fix")
	set, err := client.SetConfig(ctx, &zepb.ConfigSetRequest{
		SessionId: created.GetSessionId(),
		Path:      "bgp.peer.peer1.session.capability.route-refresh",
		Value:     "requre",
	})
	if err != nil {
		return err
	}
	if !set.GetSuccess() {
		return fmt.Errorf("gRPC SetConfig did not retain the candidate: %s", set.GetMessage())
	}
	_, err = client.CommitSession(ctx, &zepb.CommitRequest{SessionId: created.GetSessionId()})
	if err := configEnumStatus(err, codes.InvalidArgument); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: gRPC enum commit refused with InvalidArgument and the set fix last")
	after, err := client.GetRunningConfig(ctx, &zepb.Empty{})
	if err != nil {
		return err
	}
	if after.GetConfig() != before.GetConfig() {
		return fmt.Errorf("refused gRPC commit changed the running config")
	}
	diff, err := client.DiffSession(ctx, &zepb.SessionRequest{SessionId: created.GetSessionId()})
	if err != nil {
		return err
	}
	if !strings.Contains(diff.GetDiff(), "requre") {
		return fmt.Errorf("refused gRPC candidate was lost: %q", diff.GetDiff())
	}
	if _, err := client.DiscardSession(ctx, &zepb.SessionRequest{SessionId: created.GetSessionId()}); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: gRPC running config stayed unchanged and the refused candidate stayed editable")
	return nil
}

func configRefusalPath() *gpb.Path {
	return &gpb.Path{Elem: []*gpb.PathElem{
		{Name: "bgp"}, {Name: "peer", Key: map[string]string{"name": "peer1"}},
		{Name: "session"}, {Name: "capability"}, {Name: "route-refresh"},
	}}
}

func gnmiEnvironment(ctx context.Context, client gpb.GNMIClient) ([]byte, error) {
	got, err := client.Get(ctx, &gpb.GetRequest{
		Encoding: gpb.Encoding_JSON_IETF,
		Path:     []*gpb.Path{{Elem: []*gpb.PathElem{{Name: "environment"}}}},
	})
	if err != nil {
		return nil, err
	}
	if len(got.GetNotification()) != 1 || len(got.GetNotification()[0].GetUpdate()) != 1 {
		return nil, fmt.Errorf("gNMI environment Get returned no single update: %v", got)
	}
	value := got.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
	if len(value) == 0 {
		return nil, fmt.Errorf("gNMI environment Get returned no JSON value")
	}
	return value, nil
}

func gnmiSetEnumRefusal(ctx context.Context, plugin *sdk.Plugin, address, certificate string) error {
	conn, err := configRefusalClient(address, certificate)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // Closing the client changes no completed assertion.
	ctx, cancel := context.WithTimeout(ctx, WaitBudget(70, 20*time.Second))
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer secret")
	client := gpb.NewGNMIClient(conn)
	if !Poll(ctx, WaitAttempts(40, plugin01PollDelay, 40), plugin01PollDelay, func() bool {
		_, err = client.Capabilities(ctx, &gpb.CapabilityRequest{})
		return err == nil
	}) {
		return fmt.Errorf("TLS gNMI listener did not become ready: %w", err)
	}
	before, err := gnmiEnvironment(ctx, client)
	if err != nil {
		return err
	}
	beforeConfig, err := plugin01RequireDone(ctx, plugin, "show config running")
	if err != nil {
		return err
	}
	_, err = client.Set(ctx, &gpb.SetRequest{Update: []*gpb.Update{{
		Path: &gpb.Path{Elem: []*gpb.PathElem{
			{Name: "ospf"}, {Name: "redistribute", Key: map[string]string{"source": "bogus"}},
			{Name: "nssa-propagate"},
		}},
		Val: &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "true"}},
	}}})
	if err := configKeyStatus(err); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: gNMI enum key refused with InvalidArgument and the complete set fix")
	_, err = client.Set(ctx, &gpb.SetRequest{Update: []*gpb.Update{{
		Path: configRefusalPath(),
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "requre"}},
	}}})
	if err := configEnumStatus(err, codes.Aborted); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: gNMI enum Set refused with Aborted and the set fix last")
	after, err := gnmiEnvironment(ctx, client)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("refused gNMI Set changed the running environment")
	}
	afterConfig, err := plugin01RequireDone(ctx, plugin, "show config running")
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(beforeConfig, afterConfig) {
		return fmt.Errorf("refused gNMI Set changed the running config")
	}
	_, err = client.Get(ctx, &gpb.GetRequest{
		Encoding: gpb.Encoding_JSON_IETF,
		Path:     []*gpb.Path{configRefusalPath()},
	})
	if status.Code(err) != codes.NotFound {
		return fmt.Errorf("refused gNMI leaf Get code=%s, want NotFound: %w", status.Code(err), err)
	}
	fmt.Fprintln(os.Stderr, "OK: gNMI running config stayed unchanged and the refused leaf stayed absent")
	return nil
}
