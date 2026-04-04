package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goozt/gopgbase/infra/ca/internal/utils"
)

func main() {
	helperText := `Usage: ca-server [options] [certs_directory]

Options:
  -p string
		API Listener Port (default "8000")
  -h, --help
		Show this help message

Arguments:
  certs_directory
		Optional path to the directory containing the CA certificate (ca.crt).
		Alternatively, you can set the CERTS_DIR environment variable.
		If both are provided, the command-line argument takes precedence.`
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, helperText)
	}
	port := flag.String("p", "8000", "API Listener Port")
	help := flag.Bool("h", false, "show help")
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	var certsDirPath string
	if flag.NArg() > 0 {
		certsDirPath = flag.Arg(0)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	certDir := utils.GetCertDir(certsDirPath)
	utils.VerifyCertDir(certDir)

	mux := http.NewServeMux()
	registerRoutes(mux)

	handler := recoveryMiddleware(
		loggingMiddleware(
			mux,
		),
	)

	server := &http.Server{
		Addr:         ":" + *port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,   // Max time to read the request
		WriteTimeout: 10 * time.Second,  // Max time to write the response
		IdleTimeout:  120 * time.Second, // Max time for keep-alive connections
	}

	go func() {
		// logger.Info("server starting", "addr", server.Addr)
		fmt.Printf("Starting API Server\nAPI Listener: 0.0.0.0%s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	gracefulShutdown(server)
}

func gracefulShutdown(server *http.Server) {
	// Create a channel to listen for OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received
	sig := <-quit
	slog.Info("shutting down server", "signal", sig.String())

	// Give in-flight requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}
