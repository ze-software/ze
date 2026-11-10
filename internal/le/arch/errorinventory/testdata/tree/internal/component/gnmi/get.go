// Fixture: the gNMI site, a status error.
package gnmi

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// refusePath is the fixture's one gNMI site.
func refusePath() error {
	return status.Error(codes.InvalidArgument, "bad path")
}
