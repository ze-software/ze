// A table of IANA service names, two of which are also plugin names. The table
// is about services, and a gate that reads it as a copy of the plugin registry
// is reading the coincidence rather than the literal. Shaped after
// internal/core/portname/services_table.go, which holds 4954 of these.
package fixture

var services = map[string]int{
	"ssh": 22, "smtp": 25, "domain": 53, "http": 80, "pop3": 110,
	"imap": 143, "https": 443, "submission": 587, "bgp": 179, "ntp": 123,
	"ldap": 389, "syslog": 514, "rtsp": 554, "ipp": 631, "ldaps": 636,
	"rsync": 873, "ftps": 990, "telnets": 992, "imaps": 993, "pop3s": 995,
	"nntp": 119, "snmp": 161, "bootps": 67, "bootpc": 68, "tftp": 69,
	"finger": 79, "kerberos": 88, "rtelnet": 107, "auth": 113, "sftp": 115,
}
