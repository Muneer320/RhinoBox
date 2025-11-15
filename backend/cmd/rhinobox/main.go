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
	"golang.org/x/net/http2"
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

	// Configure high-performance HTTP server
	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: srv.Router(),

		// Aggressive timeouts for fast responses
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
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

	// Enable HTTP/2 with optimized settings
	http2Server := &http2.Server{
		MaxConcurrentStreams: 1000,
		MaxReadFrameSize:     1 << 20, // 1MB
		IdleTimeout:          120 * time.Second,
	}
	_ = http2.ConfigureServer(server, http2Server)

	logger.Info("starting RhinoBox",
		slog.String("addr", cfg.Addr),
		slog.String("data_dir", cfg.DataDir),
		slog.Bool("http2", true))

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
		
		// Stop job queue first (commented out - method exists but may not be needed)
		// srv.Stop()
		
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
