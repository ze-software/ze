# Learned: a dependency handed in hides the one never supplied

The PPPoE access concentrator gained a refusal counter,
`ze_pppoe_discovery_refusals_total`, so that a discovery packet Ze drops on the
wire is visible to an operator who has metrics scraping and no log access. The
counter worked in every test and existed on no PPPoE-only daemon.

## The shape

`Subsystem.Start` asked for the metrics registry and got nil:

```go
var discoveryMetrics *pppoeMetrics
if reg := registry.GetMetricsRegistry(); reg != nil {
    discoveryMetrics = initPPPoEMetrics(reg)
}
```

Nil is the normal answer there. `runYANGConfig` (`cmd/ze/hub/main.go`) runs
`engine.Start`, which runs every subsystem's `Start`, and only afterwards runs
`startStandaloneTelemetry`, which is what creates the registry on a daemon with
no `bgp` block. A BNG is exactly that daemon. So the counter was never built,
`countRefusal` no-opped for the process lifetime, and nothing was logged to say
so. The acceptance criterion said an operator sees the refusal; no operator
could.

The registry package already carries the fix, and its own comment records the
day it was paid for: every internal plugin reached its spawn with a nil registry
and silently skipped `ConfigureMetrics` forever, so `show metrics values` carried
38 series and not one came from a plugin. `InjectPluginMetrics` was written for
that, and it defers the hook until a registry exists. The PPPoE subsystem copied
the fragile half of the sibling pattern in `internal/component/l2tp/subsystem.go`
and not the deferral.

## Why every test passed

The test for the counter built its own Prometheus registry, handed it to the
server, drove a refusal and scraped the result. It is a good test of the
counting. It cannot fail for the reason the feature was broken, because it
supplies by hand the thing the daemon never supplies at all.

That is the general shape, and it is not about metrics:

**When a test hands in a dependency, it proves what the code does with the
dependency. It proves nothing about whether the dependency ever arrives.**

The two questions look like one because the same object appears in both. They
separate the moment the object's arrival is a matter of ORDER: a registry created
after `Start`, a backend loaded after a subsystem binds, a handler registered
after the reader goroutine is running. Ze is full of these, because registration
is the unifying pattern and registration is asynchronous with use.

## The test that finds it

Drive the REGISTRATION, in the arrival order the deployment produces, and assert
the dependency is bound afterwards:

```go
registry.SetMetricsRegistry(nil)
registerDiscoveryMetrics()                  // what Subsystem.Start calls
// nothing is bound yet, and counting must be a safe no-op here
registry.SetMetricsRegistry(reg)            // what the exporter does later
// the counters must exist NOW
```

`TestDiscoveryCounterBindsWhenTheRegistryArrivesLast` is that test. It was
observed red against the old shape, with the message "the discovery counters
stayed unbound after the telemetry exporter created the registry", while the
existing counter test stayed green through the same break. A red that only the
new test can produce is the proof that the new test asks a question no other
test asked.

Two things made the seam testable and both are cheap. The binding got a NAME,
`registerDiscoveryMetrics`, instead of being four lines inline in `Start`, so a
test can call the thing `Start` calls without opening a raw socket. And the
metric set moved to a package-level `atomic.Pointer`, because the store now
happens on the startup goroutine while `countRefusal` reads from the discovery
reader.

## Where to look for the next one

Grep for the read, not the write: `GetMetricsRegistry`, `GetBackend`,
`GetAuthHandler`, `GetPoolHandler`, any `Get*` on a package-level registry,
called from a `Start` or a constructor and stored in a field. Each one is a
question about ORDER that reads as a question about presence. The sibling this
one copied, `internal/component/l2tp/subsystem.go`, still has the shape; the
`bfd` plugin avoids it by re-binding at `OnStarted` as well as at
`ConfigureMetrics`, which is the same insight reached a different way.

`ai/rules/principles.md` already bans a zero that a caller cannot tell from a
failure. This is that rule at a different scale: not a zero VALUE, a zero
DEPENDENCY, with the same tell. Nothing logged, nothing red, and the feature
simply absent.

## Also from this spec

**A section with one requirement id reads as fully extracted.** RFC 2516 Section
5.4 has two MUST sentences. The compliance checklist held `RFC2516-5.4-1` for the
first, and nothing at all for the second, so `./le rfc check` asked for no test of
the Service-Name-Error reply even though the code enforced it and two tests
already asserted it. The gate can only ask about ids that exist, and an id
numbered `-1` gives no hint that a `-2` is missing. Closing it needed no new
product code: an id, a quoted sentence, two tags and two discrimination records.
When a spec's own new branch enforces an RFC sentence, check that the sentence
has an id before believing the ledger is clean.
