# p9srv

Helpers for posting plan9port namespace services.

A plan9port service is a socket at `$NAMESPACE/<name>` (Unix) or `/srv/<name>`
(Plan 9) that speaks 9P2000. Two patterns appear repeatedly across services:

| Pattern | Function |
|---------|----------|
| Direct listener — service owns its own 9P loop via `net.Listen` | [`ListenUnix`](#listenunix) |
| 9pserve proxy — service speaks 9P over a socketpair; `9pserve(1)` owns the socket | [`Post`](#post) |

Both accept a plain service name (`"acme-hotkey"`, `"init"`, …) and derive the
full socket path via [`ServicePath`](#servicepath).

## Installation

```
go get github.com/cptaffe/p9srv
```

## API

### `ListenUnix`

```go
func ListenUnix(name string) (net.Listener, func(), error)
```

Creates a Unix-domain socket at `$NAMESPACE/<name>`, removing any stale socket
from a previous run first. Returns the listener and a cleanup function.

The cleanup function closes the listener and removes the socket **only if the
path still resolves to the same inode** this call created. This prevents a
dying old instance from deleting the socket of a newer instance that has
already replaced it — a race that occurs when a supervisor restarts a service
before the previous process has fully exited.

```go
l, cleanup, err := p9srv.ListenUnix("acme-hotkey")
if err != nil {
    log.Fatal(err)
}
defer cleanup()

// also call cleanup() in the signal handler before os.Exit
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigCh
    cleanup()
    os.Exit(0)
}()

for {
    c, err := l.Accept()
    // ...
}
```

### `Post`

```go
func Post(name string) (io.ReadWriteCloser, func(), error)
```

Posts a plan9port service at `$NAMESPACE/<name>` using `9pserve(1)` (non-Plan 9)
or `srv9p.Post` (Plan 9). Returns the server end of the pipe for the caller to
serve 9P on; `9pserve` multiplexes client connections from the Unix socket onto
the pipe.

`Post` blocks until `9pserve` has created the socket, so the service is
immediately reachable by clients on return.

The cleanup function performs an inode-safe removal of the socket, then closes
the pipe and waits for `9pserve` to exit.  It returns only after `9pserve` has
fully exited.

```go
rw, cleanup, err := p9srv.Post("mysvc")
if err != nil {
    log.Fatal(err)
}
defer cleanup()

myServer.Serve(rw, rw)
```

### `ServicePath`

```go
func ServicePath(name string) string
```

Returns `filepath.Join(client.Namespace(), name)` — the canonical socket path
for a named service. Most callers can pass a name directly to `ListenUnix` or
`Post`; `ServicePath` is for code that needs the path string itself (logging,
flag defaults, diagnostics).

```go
log.Println("listening on", p9srv.ServicePath("acme-hotkey"))
```

## Plan 9

On Plan 9, `ListenUnix` is not available (`//go:build !plan9`). `Post` calls
`srv9p.Post(name)` and writes to `/srv/<name>` directly; the socketpair and
`9pserve` machinery are not used.
