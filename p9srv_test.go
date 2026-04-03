//go:build !plan9

package p9srv_test

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cptaffe/p9srv"
)

// useNamespace redirects ServicePath to a fresh temp directory for the
// duration of the test, and returns that directory.
func useNamespace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("NAMESPACE", dir)
	return dir
}

// TestServicePath checks that ServicePath joins the namespace and the name.
func TestServicePath(t *testing.T) {
	dir := useNamespace(t)
	got := p9srv.ServicePath("mysvc")
	want := filepath.Join(dir, "mysvc")
	if got != want {
		t.Errorf("ServicePath(%q) = %q; want %q", "mysvc", got, want)
	}
}

// TestListenUnix_basic checks that ListenUnix creates a connectable socket
// and that cleanup removes it.
func TestListenUnix_basic(t *testing.T) {
	dir := useNamespace(t)
	sockPath := filepath.Join(dir, "testsvc")

	l, cleanup, err := p9srv.ListenUnix("testsvc")
	if err != nil {
		t.Fatalf("ListenUnix: %v", err)
	}

	if _, err := os.Stat(sockPath); err != nil {
		t.Fatalf("socket not created: %v", err)
	}

	// Verify it accepts connections.
	accepted := make(chan error, 1)
	go func() {
		c, err := l.Accept()
		if err != nil {
			accepted <- err
			return
		}
		c.Close()
		accepted <- nil
	}()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	conn.Close()

	if err := <-accepted; err != nil {
		t.Fatalf("Accept: %v", err)
	}

	// Cleanup must remove the socket.
	cleanup()
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Errorf("socket still present after cleanup")
	}
}

// TestListenUnix_removesStale checks that a stale file at the socket path
// does not prevent ListenUnix from succeeding.
func TestListenUnix_removesStale(t *testing.T) {
	dir := useNamespace(t)
	sockPath := filepath.Join(dir, "testsvc")

	if err := os.WriteFile(sockPath, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, cleanup, err := p9srv.ListenUnix("testsvc")
	if err != nil {
		t.Fatalf("ListenUnix failed with stale file present: %v", err)
	}
	cleanup()
}

// TestListenUnix_inodeSafeCleanup checks that cleanup from an old listener
// does not remove the socket created by a newer one.
func TestListenUnix_inodeSafeCleanup(t *testing.T) {
	dir := useNamespace(t)
	sockPath := filepath.Join(dir, "testsvc")

	// First instance creates the socket.
	l1, cleanup1, err := p9srv.ListenUnix("testsvc")
	if err != nil {
		t.Fatalf("ListenUnix (first): %v", err)
	}
	l1.Close() // stop accepting, but don't call cleanup1 yet

	// Second instance replaces the socket with a new inode.
	_, cleanup2, err := p9srv.ListenUnix("testsvc")
	if err != nil {
		t.Fatalf("ListenUnix (second): %v", err)
	}
	defer cleanup2()

	// cleanup1 must not remove the new socket.
	cleanup1()
	if _, err := os.Stat(sockPath); err != nil {
		t.Errorf("cleanup1 incorrectly removed the replacement socket: %v", err)
	}
}

// TestPost_basic checks that Post creates a connectable socket and that
// cleanup removes it once 9pserve exits.  Skipped if 9pserve is not in PATH.
func TestPost_basic(t *testing.T) {
	if _, err := exec.LookPath("9pserve"); err != nil {
		t.Skip("9pserve not in PATH")
	}

	dir := useNamespace(t)
	sockPath := filepath.Join(dir, "testsvc")

	rw, cleanup, err := p9srv.Post("testsvc")
	if err != nil {
		t.Fatalf("Post: %v", err)
	}

	if rw == nil {
		t.Fatal("Post returned nil ReadWriteCloser")
	}
	// Post blocks until 9pserve has created the socket, so it is
	// immediately accessible on return.
	if _, err := os.Stat(sockPath); err != nil {
		t.Fatalf("socket not present after Post: %v", err)
	}

	// cleanup closes the pipe; cmd.Wait() ensures 9pserve has exited and
	// removed the socket before we check.
	cleanup()
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Errorf("socket still present after 9pserve exit")
	}
}
