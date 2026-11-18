// Design: docs/architecture/api/process-protocol.md -- plugin shutdown ownership
// Related: startup_removal_test.go -- live removal keeps its distinct Bye contract
package server

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Shutdown must let an engine finish its post-Run cleanup before ending the
// dependencies that cleanup uses. The interface engine releases its backend
// here; its optional VPP provider owns the transport the release still needs.
// This is full daemon shutdown, so no engine receives the removal-only Bye.
//
// The cleanup gate controls the interleaving without a sleep. Process.Done is
// the provider's existing lifetime signal: canceling it while cleanup is held
// has already authorized provider teardown, even if its goroutine has not run.
func TestShutdownKeepsDependenciesLiveThroughEngineCleanup(t *testing.T) {
	for _, chain := range []bool{false, true} {
		name := "direct-optional-provider"
		if chain {
			name = "hard-middle-optional-provider"
		}
		t.Run(name, func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })

			const (
				consumer = "z-shutdown-consumer"
				middle   = "m-shutdown-middle"
				provider = "a-shutdown-provider"
			)
			count := 2
			if chain {
				count++
			}
			type engineResult struct {
				name              string
				err               error
				consumerCompleted bool
				middleCompleted   bool
			}
			results := make(chan engineResult, count)
			cleanupEntered := make(chan struct{})
			cleanupRelease := make(chan struct{})
			consumerCompleted := make(chan struct{})
			middleCompleted := make(chan struct{})
			var releaseOnce sync.Once
			releaseCleanup := func() { releaseOnce.Do(func() { close(cleanupRelease) }) }
			var byeCalls atomic.Int32
			completed := func(done <-chan struct{}) bool {
				select {
				case <-done:
					return true
				default:
					return false
				}
			}

			run := func(name string, conn net.Conn) int {
				p := sdk.NewWithConn(name, conn)
				p.OnBye(func(string) error {
					byeCalls.Add(1)
					return nil
				})
				err := p.Run(context.Background(), &sdk.Registration{})
				result := engineResult{name: name, err: err}
				switch name {
				case consumer:
					close(cleanupEntered)
					<-cleanupRelease
					close(consumerCompleted)
				case middle:
					result.consumerCompleted = completed(consumerCompleted)
					close(middleCompleted)
				case provider:
					result.consumerCompleted = completed(consumerCompleted)
					result.middleCompleted = !chain || completed(middleCompleted)
				}
				result.err = errors.Join(result.err, p.Close())
				results <- result
				if result.err != nil {
					return 1
				}
				return 0
			}
			registerLifecyclePlugin(t, provider, nil, func(conn net.Conn) int { return run(provider, conn) })
			configs := []plugin.PluginConfig{{Name: provider, Internal: true, Encoder: plugin.EncodingJSON}}
			if chain {
				registerLifecyclePlugin(t, middle, []string{provider}, func(conn net.Conn) int { return run(middle, conn) })
				registerLifecyclePluginWithDeps(t, consumer, []string{middle}, nil, func(conn net.Conn) int { return run(consumer, conn) })
				configs = append(configs, plugin.PluginConfig{Name: middle, Internal: true, Encoder: plugin.EncodingJSON})
			} else {
				registerLifecyclePlugin(t, consumer, []string{provider}, func(conn net.Conn) int { return run(consumer, conn) })
			}
			configs = append(configs, plugin.PluginConfig{Name: consumer, Internal: true, Encoder: plugin.EncodingJSON})

			s, spawner := newLifecycleStartupServer(t)
			stopDone := make(chan struct{})
			stopStarted := false
			// Registered after the helper's cleanup, so the blocked finalizer is
			// released and joined before that helper cancels any remaining owner.
			t.Cleanup(func() {
				releaseCleanup()
				if !stopStarted {
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				select {
				case <-stopDone:
				case <-ctx.Done():
					t.Error("shutdown did not join after the cleanup gate was released")
				}
			})
			require.NoError(t, s.runPluginPhase(configs))
			providerProc := spawner.pm.GetProcess(provider)
			require.NotNil(t, providerProc)
			providerDone := providerProc.Done()
			require.NotNil(t, providerDone)
			var middleDone <-chan struct{}
			if chain {
				middleProc := spawner.pm.GetProcess(middle)
				require.NotNil(t, middleProc)
				middleDone = middleProc.Done()
				require.NotNil(t, middleDone)
			}

			guard, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			stopStarted = true
			go func() {
				s.Stop()
				s.wg.Wait()
				close(stopDone)
			}()
			select {
			case <-cleanupEntered:
			case <-guard.Done():
				t.Fatal("shutdown never reached the consumer's post-Run cleanup")
			}
			select {
			case <-providerDone:
				t.Error("shutdown canceled the provider while dependent engine cleanup still held its release gate")
			default:
			}
			if chain {
				select {
				case <-middleDone:
					t.Error("shutdown canceled the middle dependency while consumer cleanup still held its release gate")
				default:
				}
			}
			releaseCleanup()
			select {
			case <-stopDone:
			case <-guard.Done():
				t.Fatal("shutdown did not complete after consumer cleanup was released")
			}
			require.True(t, completed(consumerCompleted), "the consumer must finish its cleanup before shutdown returns")
			for range count {
				select {
				case result := <-results:
					require.NoError(t, result.err, "engine %s", result.name)
					if result.name != consumer {
						require.True(t, result.consumerCompleted, "dependency %s ended before consumer cleanup completed", result.name)
					}
					if result.name == provider {
						require.True(t, result.middleCompleted, "provider ended before middle cleanup completed")
					}
				default:
					t.Fatal("shutdown returned without every engine's finalizer result")
				}
			}
			require.Zero(t, byeCalls.Load(), "daemon shutdown must not invoke the removal-only Bye callback")
		})
	}
}
