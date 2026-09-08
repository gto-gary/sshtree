// Package reachability performs concurrent TCP-connect probes to drive a
// per-host status indicator. It's UI-framework-agnostic: CheckAll returns a
// channel of results as they arrive, which a Bubble Tea layer (or anything
// else) can wrap in its own command/message pattern.
package reachability

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	// Timeout bounds a single host's TCP-connect probe.
	Timeout = 1500 * time.Millisecond
	// DefaultPort is used when a host has no Port directive and its value
	// can't be resolved another way.
	DefaultPort = 22
	// maxConcurrent bounds simultaneous in-flight dials. Go's dialer races
	// IPv4/IPv6 per host (RFC 6555 "Happy Eyeballs"), so an unbounded run
	// can open roughly 2x len(targets) sockets at once; on macOS the
	// default per-process file descriptor limit (ulimit -n) is commonly
	// 256, well below that for a few hundred hosts. Once dials start
	// failing with "too many open files", checkHost can't tell that apart
	// from a real timeout and reports the host unreachable, so large host
	// lists showed mostly red. Capping concurrency keeps the in-flight
	// socket count well under typical OS limits regardless of host count.
	maxConcurrent = 40
)

// ResolvePort mirrors ssh_config's own port resolution: numeric strings
// parse directly; named services (e.g. "ssh") resolve via the system
// services database; anything else, or an empty value, falls back to
// DefaultPort.
func ResolvePort(value string) int {
	if value == "" {
		return DefaultPort
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n
	}
	if n, err := net.LookupPort("tcp", value); err == nil {
		return n
	}
	return DefaultPort
}

// Target is one host to probe.
type Target struct {
	Alias string
	Host  string
	Port  int
}

// Result is one completed probe.
type Result struct {
	Alias     string
	Reachable bool
}

// CheckAll dials every target concurrently, up to maxConcurrent at a time,
// and sends each Result to the returned channel as soon as it resolves —
// fastest first, not necessarily in the order targets were given — closing
// the channel once every target has reported. Cancelling ctx aborts any
// still-in-flight dials early and unblocks any goroutine currently blocked
// sending a result or waiting on the semaphore; the caller is expected to
// create a fresh, cancellable ctx per run so that re-triggering a check
// (e.g. the user pressing refresh again before the previous run finished)
// can cancel the superseded run, mirroring the exclusive-worker semantics
// the Python original relies on Textual for.
func CheckAll(ctx context.Context, targets []Target) <-chan Result {
	results := make(chan Result)
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	for _, tgt := range targets {
		wg.Add(1)
		go func(tgt Target) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			ok := checkHost(ctx, tgt.Host, tgt.Port)
			select {
			case results <- Result{Alias: tgt.Alias, Reachable: ok}:
			case <-ctx.Done():
			}
		}(tgt)
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	return results
}

func checkHost(ctx context.Context, host string, port int) bool {
	dialCtx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
