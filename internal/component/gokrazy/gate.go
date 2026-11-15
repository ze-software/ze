// Design: docs/guide/appliance.md -- "The gokrazy management UI": what the proxy forwards
// Related: gokrazy.go -- the reverse proxy this gate stands in front of
//
// The gate decides, for each request, whether the proxy may forward it with
// gokrazy's update credential. gokrazy authenticates one identity, so the
// credential opens every handler it has, the image update and the uploaded
// program runner included. Ze's own authorization is therefore the only check
// between a web account and those handlers, and the gate applies it: each
// gokrazy page is granted by the Ze command that does the same thing.

package gokrazy

import (
	"net/http"
	"path"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
)

// Authorize answers whether the account behind r may run command. isReadOnly
// selects the section of the account's authorization profile that judges it:
// run for a read, edit for anything that changes the device. The web server
// supplies it from the session's authorization.
type Authorize func(r *http.Request, command string, isReadOnly bool) bool

// The Ze commands whose authorization grants a gokrazy page. Each one does on
// the device what the page does, so a profile that allows the command allows
// the page and a profile that denies it denies the page.
const (
	// commandShowStatus shows process status, uptime and resource usage: the
	// state gokrazy's overview, status and log pages show.
	commandShowStatus = "show status"
	// commandFirmwareApply installs a new image and restarts into it, which is
	// what gokrazy's update handlers do.
	commandFirmwareApply = "update system firmware apply"
	// commandReboot restarts the device.
	commandReboot = "request reboot"
	// commandShutdown stops Ze. Stopping or restarting a supervised program on
	// the appliance, where Ze is the application, does the same.
	commandShutdown = "request shutdown"
)

// pagePath names one gokrazy handler path, or a whole subtree of them.
type pagePath struct {
	path string
	// subtree is true when path ends in "/" and names every path below it,
	// as gokrazy's own mux registers it.
	subtree bool
}

// matches reports whether requestPath is this page.
func (p pagePath) matches(requestPath string) bool {
	if p.subtree {
		return strings.HasPrefix(requestPath, p.path)
	}
	return requestPath == p.path
}

// forwardedPage is a gokrazy page the proxy forwards once the account's
// authorization grants command.
type forwardedPage struct {
	pagePath
	command string
	// readOnly is true for a page that only shows state. Only GET and HEAD
	// reach it, and command is judged in the run section.
	readOnly bool
}

// refusedPage is a gokrazy page the proxy never forwards, whatever the
// account may do, because no Ze authorization can grant what it does.
type refusedPage struct {
	pagePath
	// why says why Ze never forwards the page, and what does the job instead.
	why string
	// fix is the command that does the job instead.
	fix string
}

// The handlers come from the pinned gokrazy runtime: status.go (overview,
// status, log, assets), update.go (update, reboot, poweroff, uploadtemp,
// divert) and supervise.go (stop, restart). gokrazy registers them on one mux
// and offers no registry to read them from, so they are listed here. A path
// in neither list is refused, so a handler a newer gokrazy adds is closed
// until it is listed.
var forwardedPages = []forwardedPage{
	{path: "/", command: commandShowStatus, readOnly: true},
	{path: "/status", command: commandShowStatus, readOnly: true},
	{path: "/log", command: commandShowStatus, readOnly: true},
	{path: "/assets/", subtree: true, command: commandShowStatus, readOnly: true},
	{path: "/update/", subtree: true, command: commandFirmwareApply},
	{path: "/reboot", command: commandReboot},
	{path: "/stop", command: commandShutdown},
	{path: "/restart", command: commandShutdown},
}

// fixPushImage is the command that changes the software on the device.
const fixPushImage = "./ze-host appliance push <name>"

var refusedPages = []refusedPage{
	{
		path: "/divert",
		why: "it starts a program uploaded through the browser on the device, " +
			"outside the image the appliance was built from; " +
			"to change the software on the device, build an image and push it from the build host",
		fix: fixPushImage,
	},
	{
		path:    "/uploadtemp/",
		subtree: true,
		why: "it stores a program uploaded through the browser on the device, " +
			"for /gokrazy/divert to start outside the image the appliance was built from; " +
			"to change the software on the device, build an image and push it from the build host",
		fix: fixPushImage,
	},
	{
		path: "/poweroff",
		why: "no Ze command powers off the device, so no authorization profile can grant it; " +
			"restart the device instead",
		fix: commandReboot,
	},
}

// refusal is the answer the gate writes instead of forwarding a request.
type refusal struct {
	status int
	// allow, when set, is the Allow header a 405 answer carries.
	allow   string
	message *opererr.Error
}

func (f *refusal) write(w http.ResponseWriter) {
	if f.allow != "" {
		w.Header().Set("Allow", f.allow)
	}
	writeOperatorError(w, f.status, f.message)
}

// writeOperatorError answers status with message's text: the problem, why it
// matters, then the fix on the last line.
func writeOperatorError(w http.ResponseWriter, status int, message *opererr.Error) {
	http.Error(w, message.Error(), status)
}

// pageFor finds the page a request names, after the /gokrazy mount prefix is
// stripped. It answers the page to authorize, or the refusal to write when
// the request can never be forwarded.
func pageFor(method, requestPath string) (forwardedPage, *refusal) {
	shown := "/gokrazy" + requestPath
	if !isCleanPath(requestPath) {
		return forwardedPage{}, &refusal{
			status: http.StatusBadRequest,
			message: opererr.New(
				"bad request: "+shown+" is not a clean path",
				"Ze cannot tell which gokrazy page it names; open the gokrazy overview page",
				"/gokrazy/",
			),
		}
	}
	for i := range refusedPages {
		page := refusedPages[i]
		if page.matches(requestPath) {
			return forwardedPage{}, &refusal{
				status:  http.StatusForbidden,
				message: opererr.New("forbidden: Ze never forwards "+shown, page.why, page.fix),
			}
		}
	}
	for i := range forwardedPages {
		page := forwardedPages[i]
		if !page.matches(requestPath) {
			continue
		}
		if page.readOnly {
			if !isReadMethod(method) {
				return forwardedPage{}, &refusal{
					status: http.StatusMethodNotAllowed,
					allow:  "GET, HEAD",
					message: opererr.New(
						"method not allowed: "+shown+" is a read-only page",
						"Ze forwards only GET and HEAD to it; open the page in the browser",
						shown,
					),
				}
			}
		}
		return page, nil
	}
	return forwardedPage{}, &refusal{
		status: http.StatusNotFound,
		message: opererr.New(
			"not found: "+shown+" is not a gokrazy page Ze forwards",
			"Ze forwards only the gokrazy pages its authorization can judge; open the gokrazy overview page",
			"/gokrazy/",
		),
	}
}

// unauthorized is the refusal for an account whose profile does not grant the
// command a page needs. The fix is that command, for an administrator to allow.
func unauthorized(requestPath, command string) *refusal {
	return &refusal{
		status: http.StatusForbidden,
		message: opererr.New(
			"forbidden: this account may not open /gokrazy"+requestPath,
			"your authorization profile does not grant the Ze command the page needs; "+
				"ask an administrator to allow this command in your authorization profile",
			command,
		),
	}
}

// isReadMethod reports whether method only reads.
func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// isCleanPath reports whether requestPath is already in the form path.Clean
// gives it, a trailing slash aside. The gate judges the path it forwards, so
// a path gokrazy's mux would rewrite (a "..", a doubled slash) is refused
// rather than judged as one page and served as another.
func isCleanPath(requestPath string) bool {
	cleaned := path.Clean(requestPath)
	if cleaned == "/" {
		return requestPath == "/"
	}
	if strings.HasSuffix(requestPath, "/") {
		cleaned += "/"
	}
	return cleaned == requestPath
}
