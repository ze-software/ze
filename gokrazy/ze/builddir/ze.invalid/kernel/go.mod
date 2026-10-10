module gokrazy/build/ze

go 1.26.2

// ze.invalid/kernel is ze's runtime kernel, assembled per build from the
// resolved kernel cache entry. Instance preparation adds the replace that
// points this require at it (internal/appliance/instance, Prepare); the path
// never resolves without that replace, by design.
require ze.invalid/kernel v0.0.0
