//go:build !plan9

package p9srv

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

// ListenUnix creates a Unix-domain socket at $NAMESPACE/<name> for a
// plan9port service, removing any stale socket from a previous run first.
//
// The returned cleanup function closes the listener and removes the socket
// only if $NAMESPACE/<name> still resolves to the same inode that this call
// created.  This prevents a dying old instance from deleting the socket of a
// newer instance that has already replaced it — a race that occurs when
// supervisor restarts overlap with slow process exits.
func ListenUnix(name string) (net.Listener, func(), error) {
	path := ServicePath(name)
	os.Remove(path) // remove stale socket from a previous run

	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, nil, err
	}

	ino := socketIno(path)
	cleanup := func() {
		l.Close()
		if ino != 0 && socketIno(path) == ino {
			os.Remove(path)
		}
	}
	return l, cleanup, nil
}

// socketIno returns the inode number of the Unix socket at path, or 0 if
// path does not exist or is not a socket.
func socketIno(path string) uint64 {
	fi, err := os.Stat(path)
	if err != nil || fi.Mode()&os.ModeSocket == 0 {
		return 0
	}
	return fi.Sys().(*syscall.Stat_t).Ino
}

// waitSocket polls path until a socket appears there or the timeout elapses.
func waitSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if socketIno(path) != 0 {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for socket %s", path)
}
