package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang-gin/internal/config"
	"golang-gin/internal/handler"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
	log.Println("stopped")
}

func run() error {
	addr, err := config.HTTPAddr()
	if err != nil {
		return err
	}

	// Cancelled on SIGINT/SIGTERM, which is what ends the wait below.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lc := net.ListenConfig{KeepAlive: 3 * time.Minute}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Handler:           handler.NewEngine(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		// ln.Addr() is what the socket actually bound to, so ":0" reports the
		// real port and no environment value reaches the log.
		log.Printf("listening on %s", ln.Addr())
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		stop() // a second signal now kills the process instead of being swallowed
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
