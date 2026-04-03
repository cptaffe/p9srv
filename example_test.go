//go:build !plan9

package p9srv_test

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cptaffe/p9srv"
)

func ExampleListenUnix() {
	l, cleanup, err := p9srv.ListenUnix("acme-hotkey")
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	// Also call cleanup() in the signal handler so the socket is removed
	// promptly; defer alone only runs after main returns.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cleanup()
		os.Exit(0)
	}()

	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		_ = c
	}
}

func ExamplePost() {
	rw, cleanup, err := p9srv.Post("mysvc")
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	// Serve 9P on the pipe; 9pserve multiplexes client connections onto it.
	_ = rw
}

func ExampleServicePath() {
	log.Println("listening on", p9srv.ServicePath("acme-hotkey"))
}
