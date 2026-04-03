//go:build !plan9

package p9srv

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

// Post posts a plan9port service named name at $NAMESPACE/<name> using
// 9pserve(1).
//
// It removes any stale socket at that path, creates a socketpair, starts
// 9pserve with the child end as its stdio, and returns the parent end for
// the caller to serve 9P on.  9pserve multiplexes client connections from
// the Unix socket onto the pipe.
//
// The cleanup function closes the parent end; 9pserve receives EOF and
// exits, removing the socket file it owns.
//
// Typical usage:
//
//	rw, cleanup, err := p9srv.Post("mysvc")
//	if err != nil { log.Fatal(err) }
//	defer cleanup()
//	myServer.Serve(rw)
func Post(name string) (io.ReadWriteCloser, func(), error) {
	path := ServicePath(name)
	os.Remove(path) // remove stale socket from a previous run

	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("socketpair: %w", err)
	}
	// Set FD_CLOEXEC on both ends.  macOS exposes SOCK_CLOEXEC in the
	// header but x/sys/unix does not export it for darwin, so we use a
	// separate call.  The child end survives exec because exec.Cmd dup2s
	// it onto fd 0 and fd 1 before exec, and dup2 clears FD_CLOEXEC on
	// the destination fd.
	unix.CloseOnExec(fds[0])
	unix.CloseOnExec(fds[1])
	parent := os.NewFile(uintptr(fds[0]), "p9srv-parent")
	child := os.NewFile(uintptr(fds[1]), "p9srv-child")

	cmd := exec.Command("9pserve", "unix!"+path)
	cmd.Stdin = child
	cmd.Stdout = child
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		parent.Close()
		child.Close()
		return nil, nil, fmt.Errorf("9pserve: %w", err)
	}
	child.Close() // parent holds the only remaining reference to its end

	cleanup := func() {
		parent.Close() // 9pserve gets EOF and exits, removing the socket
		cmd.Wait()     //nolint:errcheck
	}
	return parent, cleanup, nil
}
