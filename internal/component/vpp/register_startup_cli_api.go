// Design: docs/guide/vpp.md -- one typed native CLI binding shared by callers
// Related: startup_cli_api.go -- narrow pinned-schema request and reply
// GoVPP v0.13.0: https://github.com/FDio/govpp/blob/v0.13.0/api/binapi.go
package vpp

import "go.fd.io/govpp/api"

func init() {
	api.RegisterMessage((*startupCLIRequest)(nil), "cli_inband_f8377302")
	api.RegisterMessage((*startupCLIReply)(nil), "cli_inband_reply_05879051")
}
