package fixture

// The registration lives in another file of the same package, which is why the
// judgement is per package rather than per file.
func init() {
	for _, meta := range []diagnostic.CodeMeta{
		{Code: codeTLSExpired, Summary: "the certificate has expired"},
		{Code: codeTLSMissing, Summary: "no certificate is configured"},
	} {
		_ = diagnostic.Register(meta)
	}
}
