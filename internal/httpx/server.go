package httpx

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
)

type ServerTimeouts struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func NewServer(addr string, handler http.Handler, t ServerTimeouts) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: t.ReadHeaderTimeout,
		ReadTimeout:       t.ReadTimeout,
		WriteTimeout:      t.WriteTimeout,
		IdleTimeout:       t.IdleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return context.Background()
		},
	}
}

// Запускает HTTP-сервер и корректно завершает его по SIGINT/SIGTERM
func RunHTTPServer(addr string, handler http.Handler, shutdownTimeout time.Duration) error {
	srv := NewServer(addr, handler, ServerTimeouts{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	})

	return RunHTTPServers(shutdownTimeout, srv)
}

// Запускает несколько HTTP-серверов и корректно завершает их по SIGINT/SIGTERM
func RunHTTPServers(shutdownTimeout time.Duration, servers ...*http.Server) error {
	if len(servers) == 0 {
		return nil
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, len(servers))

	for _, srv := range servers {
		s := srv
		go func() {
			log.Printf("HTTP сервер слушает %s", s.Addr)
			err := s.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
				return
			}
			errCh <- nil
		}()
	}

	select {
	case <-stop:
		log.Printf("Получен сигнал остановки, завершаем HTTP серверы...")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		shutdownAll(ctx, servers)
		return firstNonNil(errCh, len(servers))

	case err := <-errCh:
		if err != nil {
			ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()
			shutdownAll(ctx, servers)
			return err
		}
		return firstNonNil(errCh, len(servers)-1)
	}
}

func shutdownAll(ctx context.Context, servers []*http.Server) {
	for _, srv := range servers {
		_ = srv.Shutdown(ctx)
	}
}

func firstNonNil(errCh <-chan error, count int) error {
	var first error
	for range count {
		if err := <-errCh; err != nil && first == nil {
			first = err
		}
	}
	return first
}
