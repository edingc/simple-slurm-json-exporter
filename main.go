package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/common/promslog"
	promslogflag "github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

var (
	scrapeTimeout = kingpin.Flag("slurm.scrape-timeout", "Maximum time to wait for a slurm command to return.").Default("30s").Duration()
	toolkitFlags  = kingpinflag.AddFlags(kingpin.CommandLine, ":9427")
)

func main() {
	promslogConfig := &promslog.Config{}
	promslogflag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print("simple-slurm-json-exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(promslogConfig)
	slog.SetDefault(logger)
	logger.Info("Starting simple-slurm-json-exporter", "version", version.Info())

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/squeue", squeueHandler(*scrapeTimeout, logger))

	landingConfig := web.LandingConfig{
		Name:        "Simple Slurm JSON Exporter",
		Description: "Exposes JSON output of Slurm commands via HTTP for scraping.",
		Version:     version.Info(),
		Links: []web.LandingLinks{
			{Address: "/squeue", Text: "squeue"},
			{Address: "/health", Text: "Health"},
		},
	}
	landingPage, err := web.NewLandingPage(landingConfig)
	if err != nil {
		logger.Error("Error building landing page", "err", err)
		os.Exit(1)
	}
	mux.Handle("/", landingPage)

	srv := &http.Server{Handler: mux}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		if err := web.ListenAndServe(srv, toolkitFlags, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Error starting HTTP server", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during server shutdown", "err", err)
	}
}

func squeueHandler(timeout time.Duration, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		cmdOut, err := exec.CommandContext(ctx, "squeue", "--json").Output()
		if err != nil {
			logger.Error("squeue failed", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "failed to run squeue")
			return
		}

		if !json.Valid(cmdOut) {
			logger.Error("squeue returned invalid JSON", "bytes", len(cmdOut))
			writeJSONError(w, http.StatusInternalServerError, "squeue returned invalid JSON")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(cmdOut); err != nil {
			logger.Warn("write response failed", "err", err)
		}
	}
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
