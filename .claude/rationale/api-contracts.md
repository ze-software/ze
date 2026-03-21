# Rationale: API Contracts in Comments

## Problem

`TestProcessInternalPluginStop` was intermittently failing due to a data race. Root cause: `TestProcessInternalPlugin` called `Stop()` without `Wait()`, leaking a goroutine that raced with the next test. Nothing in the `Stop()` comment mentioned that `Wait()` was required.

The obligation was implicit knowledge -- obvious to the author, invisible to the next reader.

## Principle

If skipping a function call causes a leak, deadlock, or race, the godoc must say so. Comments are the only documentation that travels with the code. Architecture docs get stale; function comments get read at point of use.

## Why Both Sides

Documenting the obligation only on `Stop()` ("call Wait after") helps someone reading Stop. Documenting it only on `Wait()` ("must follow Stop") helps someone reading Wait. Neither alone helps someone who only reads the type doc or constructor. All three must state the contract so any entry point into the API reveals it.
