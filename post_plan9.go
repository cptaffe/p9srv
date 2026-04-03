//go:build plan9

package p9srv

import (
	"fmt"
	"io"

	"9fans.net/go/plan9/srv9p"
)

// Post posts a plan9port service named name to /srv/<name> using srv9p.Post.
// The cleanup function closes the pipe.
func Post(name string) (io.ReadWriteCloser, func(), error) {
	rw, err := srv9p.Post(name)
	if err != nil {
		return nil, nil, fmt.Errorf("post %s: %w", name, err)
	}
	return rw, func() { rw.Close() }, nil
}
