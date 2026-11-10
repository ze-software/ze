package web

import "net/http"

// refuseUnauthorized is the fixture's one web surface site: a plain refusal.
func refuseUnauthorized(w http.ResponseWriter) {
	http.Error(w, "unauthorized", 401)
}
