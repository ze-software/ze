// Design: docs/labs/pppoe-interop.md -- observe a complete original HTTP reply.
// Related: product_state_test.go -- existing authenticated HTTP transport fixture.
package testvpppppoe

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type productReplyCloseBody struct {
	*strings.Reader
	closeFailure error
	closes       int
}

func (body *productReplyCloseBody) Close() error {
	body.closes++
	return body.closeFailure
}

// Goal: a valid original reply does not hide failure to close that reply.
// Method: drive the real observer through its retained authenticated transport,
// with successful and failed body closure; the original error must survive.
func TestProductObserverRetainsOriginalBodyCloseFailure(t *testing.T) {
	failure := errors.New("original product reply close failed")
	for _, test := range []struct {
		name         string
		closeFailure error
	}{
		{name: "closed"},
		{name: "close failure", closeFailure: failure},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := &productReplyCloseBody{Reader: strings.NewReader("{\"status\":\"done\",\"data\":[]}"), closeFailure: test.closeFailure}
			client := &http.Client{Transport: productHTTPTransport(func(request *http.Request) (*http.Response, error) {
				if request.URL.Host != "127.0.0.1:9099" {
					t.Fatal("observer changed its original scoped endpoint")
				}
				if request.Header.Get("Authorization") != "Bearer "+productToken {
					t.Fatal("observer lost its original authentication")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
			})}
			data, err := observeProduct(context.Background(), client, "show pppoe sessions")
			if !errors.Is(err, test.closeFailure) {
				t.Fatalf("original body close cause=%v, expected %v", err, test.closeFailure)
			}
			if body.closes != 1 {
				t.Fatalf("original reply body closes=%d", body.closes)
			}
			if string(data) != "[]" {
				t.Fatalf("original valid table changed: %s", data)
			}
		})
	}
}
