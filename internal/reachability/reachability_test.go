package reachability

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestResolvePort(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  int
	}{
		{"empty defaults to 22", "", DefaultPort},
		{"numeric", "2222", 2222},
		{"named service ssh", "ssh", 22}, // resolves via services db, or falls back to DefaultPort — both give 22
		{"unknown falls back", "not-a-real-service-xyz", DefaultPort},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolvePort(tc.value); got != tc.want {
				t.Errorf("ResolvePort(%q) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

// openListenerPort starts a listener on an OS-assigned loopback port and
// returns its port number, leaving it open so a dial against it succeeds.
func openListenerPort(t *testing.T) (int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln.Addr().(*net.TCPAddr).Port, func() { ln.Close() }
}

// closedPort returns a loopback port number guaranteed to have nothing
// listening on it, so a dial against it is refused quickly.
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestCheckAllReportsUpAndDown(t *testing.T) {
	upPort, closeListener := openListenerPort(t)
	defer closeListener()
	downPort := closedPort(t)

	targets := []Target{
		{Alias: "up-host", Host: "127.0.0.1", Port: upPort},
		{Alias: "down-host", Host: "127.0.0.1", Port: downPort},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got := map[string]bool{}
	for r := range CheckAll(ctx, targets) {
		got[r.Alias] = r.Reachable
	}

	if !got["up-host"] {
		t.Error("up-host: want reachable, got unreachable")
	}
	if got["down-host"] {
		t.Error("down-host: want unreachable, got reachable")
	}
}

func TestCheckAllClosesChannelWhenDone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count := 0
	for range CheckAll(ctx, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("expected no results for empty target list, got %d", count)
	}
}

func TestCheckAllRespectsCancellation(t *testing.T) {
	// A non-routable address (TEST-NET-1, RFC 5737) that should hang rather
	// than immediately refuse, so cancelling ctx is what ends the check.
	ctx, cancel := context.WithCancel(context.Background())
	targets := []Target{{Alias: "unreachable", Host: "192.0.2.1", Port: 22}}

	ch := CheckAll(ctx, targets)
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			// A result may still race in before cancellation is observed;
			// draining is fine either way as long as the channel closes.
			for range ch {
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("CheckAll did not respect context cancellation within 3s")
	}
}
