# Roadmap

Ze is pre-release. This page is about one thing: the work that stands between today and a first release you can trust in production. It is a sequence, not a calendar. Each step gates the next, and the honest answer to "when" is "when the step before it is genuinely done".

The [changes log](https://ze-software.net/project/changes/) records what has already shipped week by week. This page looks forward.

## 1. Close the open specs

Ze is built spec by spec: nothing gets written without a spec that says what it should do and how it will be tested. There is still a backlog of open specs, and the first job is to work through it so the feature set stops being a moving target. Until that is done, the surface keeps growing, and a growing surface is the wrong thing to try to stabilise.

`Specs` `Tests` `Scope control`

## 2. Make it stable

Once the feature set stops moving, the effort turns to hardening what is there: chasing down races, tightening the tests, and closing the known trust-model gaps tracked in the [security policy](https://ze-software.net/security/). The routing core is already heavily tested, but "heavily tested" and "boring to run for months" are not the same thing, and the second one is the goal.

`Security` `Routing core` `Reliability`

## 3. Prove it in production

Tests and lab interop take Ze a long way, but they are not a substitute for real traffic on real hardware for long enough to find operational failures. This step is about running Ze where it has to keep working, watching what breaks, and fixing it. Operator feedback matters most here, which is why the project is open this early.

`Real traffic` `Telemetry` `Hardware`

## 4. Freeze the configuration syntax

Today the configuration syntax can still change, and the site says so. Before a release, the syntax has to settle so that a config you write keeps working. The policy is no silent breakage: any change that affects an existing config comes with either an automatic migration or a clear error. Getting the syntax to a point where it stops needing to change is the last big piece before release.

`Migration` `Validation` `No silent breakage`

## 5. Release

The first release is what all of the above adds up to: a frozen configuration model, a stable feature set, and enough production mileage to stand behind it. Upgrade paths will be provided from that point on.

`Frozen model` `Production evidence` `Upgrade paths`

## What is not the priority right now

New large features are not the focus on the way to the first release. Stability comes first. Known gaps against other daemons, such as BGP confederations, are listed honestly on the [comparison page](https://ze-software.net/compare/), and the [features page](https://ze-software.net/features/) marks what is shipped versus still experimental. If something you need is missing or you want to help move a milestone along, say so on [Discord](https://discord.gg/T8s7CjPDne) or the [issue tracker](https://github.com/ze-software/ze/issues).

`Not now` `Known gaps` `Feedback`
