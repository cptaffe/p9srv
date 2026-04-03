//go:build !plan9

package p9srv

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

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
// Post blocks until 9pserve has created the socket, so the service is
// immediately reachable by clients upon return.
//
// The cleanup function performs an inode-safe removal of the socket and then
// closes the pipe; 9pserve receives EOF and exits.  Cleanup is synchronous:
// it returns only after 9pserve has fully exited.
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

	// Block until 9pserve creates the socket so callers can connect
	// immediately after Post returns.
	if err := waitSocket(path, 5*time.Second); err != nil {
		parent.Close()
		cmd.Wait() //nolint:errcheck
		return nil, nil, err
	}

	// Capture the inode now so cleanup can remove the socket safely.  We
	// remove it ourselves rather than relying on 9pserve's atexit handler,
	// which runs asynchronously relative to cmd.Wait.  The inode check
	// prevents a dying old instance from removing a replacement socket
	// created by a newer one.
	ino := socketIno(path)

	cleanup := func() {
		if ino != 0 && socketIno(path) == ino {
			os.Remove(path)
		}
		parent.Close() // 9pserve gets EOF on stdin and exits
		cmd.Wait()     //nolint:errcheck
	}
	return parent, cleanup, nil
}
