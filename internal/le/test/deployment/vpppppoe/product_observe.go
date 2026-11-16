// Design: docs/labs/pppoe-interop.md -- observe the real daemon through its REST API.
// Related: product_check.go -- this helper does not manufacture daemon state.
package testvpppppoe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

const (
	productHelper     = "/product/le"
	productToken      = "ze-vpp-product" // #nosec G101 -- isolated loopback fixture credential.
	productReplyLimit = 1 << 20
)

// productObservation uses le's shared Prose renderer. Docker exec therefore
// receives exactly the API data JSON, without a private stdout implementation.
type productObservation struct {
	Data json.RawMessage `json:"data"`
}

func (observation productObservation) Text() string { return string(observation.Data) + "\n" }

func answerProductObserve(args leaction.Arguments) (any, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := observeProduct(ctx, http.DefaultClient, args.One("command"))
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the product lab cannot observe Ze", "An unavailable or incomplete daemon answer cannot establish session state.", "./le test deployment vpp-pppoe product"))
		return nil, 1
	}
	return productObservation{Data: data}, 0
}

// observeProduct has a fixed loopback destination and read-only command allowlist.
// The parent product action owns admission. This helper performs one bounded
// authenticated HTTP request; it starts no process and opens no PPP socket.
func observeProduct(ctx context.Context, client *http.Client, command string) (data json.RawMessage, err error) {
	if command != "show pppoe sessions" && command != "show pppoe interfaces" {
		return nil, fmt.Errorf("unsupported product observation %q", command)
	}
	body, err := json.Marshal(struct {
		Command string `json:"command"`
	}{Command: command})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:9099/api/v1/execute", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+productToken)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, response.Body.Close()) }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the Ze REST returned HTTP %d", response.StatusCode)
	}
	wire, err := io.ReadAll(io.LimitReader(response.Body, productReplyLimit+1))
	if err != nil {
		return nil, err
	}
	if len(wire) > productReplyLimit {
		return nil, errors.New("the Ze REST reply exceeds the product observer bound")
	}
	var answer struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(wire, &answer); err != nil {
		return nil, fmt.Errorf("decode Ze REST answer: %w", err)
	}
	if answer.Status != "done" || answer.Error != "" {
		return nil, fmt.Errorf("the Ze REST command did not complete: status=%q error=%q", answer.Status, answer.Error)
	}
	if len(answer.Data) == 0 {
		return nil, errors.New("the Ze REST command omitted its data member")
	}
	if bytes.Equal(bytes.TrimSpace(answer.Data), []byte("null")) {
		return nil, errors.New("the Ze REST command returned null data")
	}
	return answer.Data, nil
}
