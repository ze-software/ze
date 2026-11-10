// Fixture: the REST site, an error body.
package rest

import "net/http"

// refuseRequest is the fixture's one REST site.
func refuseRequest(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "bad request")
}
