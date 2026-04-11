module gokrazy/build/ze

go 1.26.2

require github.com/rtr7/kernel v0.0.0-20260403073601-5a996da3a37b // indirect

// Force the current gokrazy version so the stale transitive x/crypto / x/net / x/sys
// copies shipped with the 2022 pin are not pulled into gokrazy/modcache/. The
// rtr7/kernel package carries gokrazy as an indirect dep only; no Go source in
// the kernel package imports the gokrazy module, so replacement is safe.
replace github.com/gokrazy/gokrazy v0.0.0-20220304072708-5dd8496371e3 => github.com/gokrazy/gokrazy v0.0.0-20260218074004-791851666ca2
