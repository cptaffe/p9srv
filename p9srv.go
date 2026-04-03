// Package p9srv provides helpers for plan9port namespace services.
//
// A plan9port service is a socket posted at $NAMESPACE/<name> (Unix) or
// /srv/<name> (Plan 9) that speaks 9P2000.  Two posting patterns are common:
//
//  1. Direct listener — the service manages its own 9P loop over a
//     Unix-domain socket it creates directly.  Use [ListenUnix].
//
//  2. 9pserve proxy — the service speaks 9P over a socketpair; 9pserve(1)
//     multiplexes clients onto that pipe and owns the socket file.
//     Use [Post].  On Plan 9, [Post] calls srv9p.Post instead.
//
// Both functions accept a plain service name ("acme-hotkey", "init", …)
// and derive the full path internally via [ServicePath].
package p9srv

import (
	"path/filepath"

	"9fans.net/go/plan9/client"
)

// ServicePath returns the canonical filesystem path for a named service in
// the current plan9port namespace: $NAMESPACE/<name> on Unix.
//
// Most callers can use [ListenUnix] or [Post] directly; ServicePath is
// exposed for code that needs the path string itself (logging, diagnostics).
func ServicePath(name string) string {
	return filepath.Join(client.Namespace(), name)
}
