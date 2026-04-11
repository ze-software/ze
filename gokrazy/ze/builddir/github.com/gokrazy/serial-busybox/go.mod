module gokrazy/build/ze

go 1.26.2

require github.com/gokrazy/serial-busybox v0.0.0-20250119153030-ac58ba7574e7 // indirect

// Force the current gokrazy version so the stale transitive x/crypto / x/sys copies
// shipped with the 2020 pin are not pulled into gokrazy/modcache/. serial-busybox
// only calls gokrazy.DontStartOnBoot(), which exists in both the 2020 and the 2026
// gokrazy with the same signature, so replacement is safe at the API level.
replace github.com/gokrazy/gokrazy v0.0.0-20200501080617-f3445e01a904 => github.com/gokrazy/gokrazy v0.0.0-20260218074004-791851666ca2
