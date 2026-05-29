package updateflag

import (
	"net/url"
	"strings"
)

type Value struct {
	Update string
}

func (v Value) GetUpdateTarget(hostname string) (defaultPassword, updateHostname string) {
	if v.Update == "" {
		// -update not set
		return "", hostname
	}
	if v.Update == "yes" {
		// -update=yes
		return "", hostname
	}
	if strings.HasPrefix(v.Update, ":") {
		// port number syntax, e.g. -update=:2080
		return "", hostname
	}
	// -update=<url> syntax
	u, err := url.Parse(v.Update)
	if err != nil {
		return "", hostname
	}
	defaultPassword, _ = u.User.Password()
	return defaultPassword, u.Host
}

func (v Value) BaseURL(httpPort, httpsPort, schema, hostname, pw string) (*url.URL, error) {
	if v.Update != "yes" && !strings.HasPrefix(v.Update, ":") {
		// already fully qualified, nothing to add
		return url.Parse(v.Update)
	}
	port := httpPort
	defaultPort := "80"
	if schema == "https" {
		port = httpsPort
		defaultPort = "443"
	}
	if strings.HasPrefix(v.Update, ":") {
		port = strings.TrimPrefix(v.Update, ":")
	}
	v.Update = schema + "://gokrazy:" + pw + "@" + hostname
	if port != defaultPort {
		v.Update += ":" + port
	}
	v.Update += "/"
	return url.Parse(v.Update)
}
