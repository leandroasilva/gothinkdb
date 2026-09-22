package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/leandroasilva/gothinkdb/internal/api"
	"github.com/leandroasilva/gothinkdb/internal/config"
	"github.com/leandroasilva/gothinkdb/internal/protocol"
	"github.com/leandroasilva/gothinkdb/internal/reql"
	"github.com/leandroasilva/gothinkdb/internal/rpc"
)

//go:embed all:dashboard
var dashboardFS embed.FS

func main() {
	// Parse command line flags
	configFile := flag.String("config", "", "Path to configuration file")
	dataDir := flag.String("data", "/data/gothinkdb", "Data directory path")
	driverAddr := flag.String("driver-address", ":28015", "Driver protocol listen address")
	httpAddr := flag.String("http-address", ":8080", "HTTP admin interface address")
	clusterAddr := flag.String("cluster-address", ":29015", "Cluster communication address")
	joinAddr := flag.String("join", "", "Address of existing cluster node to join")
	serverName := flag.String("server-name", "", "Server name (auto-generated if empty)")
	flag.Parse()

	// Load configuration
	cfg := config.Load(*configFile)
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *driverAddr != "" {
		cfg.DriverAddress = *driverAddr
	}
	if *httpAddr != "" {
		cfg.HTTPAddress = *httpAddr
	}
	if *clusterAddr != "" {
		cfg.ClusterAddress = *clusterAddr
	}
	if *joinAddr != "" {
		cfg.JoinAddress = *joinAddr
	}
	if *serverName != "" {
		cfg.ServerName = *serverName
	}

	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting GoThinkDB",
		"data_dir", cfg.DataDir,
		"driver_address", cfg.DriverAddress,
		"http_address", cfg.HTTPAddress,
		"cluster_address", cfg.ClusterAddress,
	)

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		slog.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	// Create core components
	evaluator := reql.NewEvaluator()
	cluster := rpc.NewClusterManager(cfg.ServerName)

	// Create API server with all routes
	apiServer := api.NewServer(evaluator, cluster)

	// Setup dashboard file server from embedded FS with SPA fallback
	dashboardSub, err := fs.Sub(dashboardFS, "dashboard")
	if err != nil {
		slog.Error("failed to create dashboard sub filesystem", "error", err)
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(dashboardSub))

	// SPA fallback handler: serve index.html for client-side routes
	spaHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested file
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		f, err := dashboardSub.Open(strings.TrimPrefix(path, "/"))
		if err != nil {
			// File not found - serve index.html for SPA routing
			indexFile, err := dashboardSub.Open("index.html")
			if err != nil {
				http.Error(w, "index.html not found", http.StatusInternalServerError)
				return
			}
			defer indexFile.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.Copy(w, indexFile)
			return
		}
		f.Close()
		// File exists, serve it
		fileServer.ServeHTTP(w, r)
	})

	// Create main mux that combines API + dashboard
	mux := http.NewServeMux()

	// Register all API routes via api.Server
	mux.Handle("/api/", apiServer)

	// Serve dashboard with SPA fallback
	mux.Handle("/", spaHandler)

	httpServer := &http.Server{
		Addr:         cfg.HTTPAddress,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in goroutine
	go func() {
		slog.Info("HTTP admin server starting", "address", cfg.HTTPAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	// Start protocol server (ReQL) - share the same evaluator as the API server
	handler := protocol.NewReQLHandler(evaluator)
	protocolServer := protocol.NewServer(cfg.DriverAddress, handler)
	if err := protocolServer.Start(); err != nil {
		slog.Error("failed to start protocol server", "error", err)
		os.Exit(1)
	}
	slog.Info("ReQL protocol server started successfully", "address", cfg.DriverAddress)

	slog.Info("GoThinkDB server started successfully")
	fmt.Printf("\nGoThinkDB is ready!\n")
	fmt.Printf("  Dashboard:       http://localhost%s\n", cfg.HTTPAddress)
	fmt.Printf("  Driver protocol: localhost%s\n", cfg.DriverAddress)
	fmt.Printf("  Cluster:         localhost%s\n", cfg.ClusterAddress)
	fmt.Printf("  Health check:    http://localhost%s/api/health\n\n", cfg.HTTPAddress)

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down GoThinkDB...")

	// Stop protocol server
	if err := protocolServer.Stop(); err != nil {
		slog.Error("protocol server shutdown error", "error", err)
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("HTTP server forced to shutdown", "error", err)
	}

	slog.Info("GoThinkDB stopped")
}
