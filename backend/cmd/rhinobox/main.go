package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		panic(err)
	}
	defer srv.Stop()

	// Allow bounded local uploads time to finish without claiming plaintext HTTP/2.
	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: srv.Router(),

		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,

		// High connection limits
		MaxHeaderBytes: 1 << 20, // 1MB

		// Optimized connection settings
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			// Enable TCP keepalive
			if tc, ok := c.(*net.TCPConn); ok {
				tc.SetKeepAlive(true)
				tc.SetKeepAlivePeriod(30 * time.Second)
			}
			return ctx
		},
	}

	logger.Info("starting RhinoBox",
		slog.String("addr", cfg.Addr),
		slog.String("data_dir", cfg.DataDir))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.String("addr", cfg.Addr))
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down gracefully...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown error", slog.Any("err", err))
			os.Exit(1)
		}
		logger.Info("server stopped")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.Any("err", err))
			os.Exit(1)
		}
	}
}
