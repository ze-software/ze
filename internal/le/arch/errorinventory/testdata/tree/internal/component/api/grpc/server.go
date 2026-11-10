// Fixture: the gRPC command API site, a status error outside gNMI.
package grpc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// refuseCommand is the fixture's one gRPC site.
func refuseCommand() error {
	return status.Error(codes.Unavailable, "engine down")
}
