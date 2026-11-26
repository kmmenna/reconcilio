package main

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/ingestion"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/telemetry"
	"github.com/kmmenna/reconcilio/apps/reconciliation/migrations"
)

func main() {
	os.Exit(commandExitCode())
}

func commandExitCode() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	traces, traceErr := stdoutEnabled("RECONCILIO_TRACE_STDOUT")
	metrics, metricErr := stdoutEnabled("RECONCILIO_METRIC_STDOUT")
	signals, err := telemetry.New(telemetry.Options{Traces: traces, Metrics: metrics})
	if err != nil {
		slog.Error("Could not start telemetry", "error", err)
		return 1
	}
	slog.SetDefault(signals.Logger)
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := signals.Shutdown(shutdown); err != nil {
			fmt.Fprintln(os.Stderr, "Could not shut down telemetry:", err)
		}
	}()
	if err := errors.Join(traceErr, metricErr); err != nil {
		slog.Error("Invalid telemetry configuration", "setting", err.Error())
		return 1
	}

	command := "serve"
	if len(os.Args) == 2 {
		command = os.Args[1]
	}
	if len(os.Args) > 2 {
		slog.Error("Expected one command from serve, migrate, or bootstrap-admin")
		return 1
	}
	if err := run(ctx, command, signals); err != nil {
		slog.Error("Reconcilio command failed", "command", command, "error", err)
		return 1
	}
	return 0
}

func stdoutEnabled(name string) (bool, error) {
	enabled, err := strconv.ParseBool(cmp.Or(os.Getenv(name), "false"))
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return enabled, nil
}

func run(ctx context.Context, command string, signals *telemetry.Runtime) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("configure database: %w", err)
	}
	defer pool.Close()

	switch command {
	case "migrate":
		return migrations.Apply(ctx, pool)
	case "bootstrap-admin":
		return bootstrapAdmin(ctx, pool)
	case "serve":
		return serve(ctx, pool, signals)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func bootstrapAdmin(ctx context.Context, pool *pgxpool.Pool) error {
	accounts := identity.New(pool, nil)
	password, created, err := accounts.BootstrapAdmin(ctx)
	if err != nil {
		return err
	}
	if created {
		fmt.Println("Administrator username: admin")
		fmt.Printf("Generated password: %s\n", password)
	} else {
		fmt.Println("Administrator is already configured.")
	}
	return nil
}

func serve(ctx context.Context, pool *pgxpool.Pool, signals *telemetry.Runtime) error {
	apiEnabled, err := componentEnabled("RECONCILIO_API_ENABLED")
	if err != nil {
		return err
	}
	workerEnabled, err := componentEnabled("RECONCILIO_WORKER_ENABLED")
	if err != nil {
		return err
	}
	if !apiEnabled && !workerEnabled {
		return errors.New("enable the API, the worker, or both")
	}
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	stopMetrics, err := signals.ObserveBusinessMetrics(pool)
	if err != nil {
		return err
	}
	defer func() {
		flush, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := errors.Join(signals.FlushMetrics(flush), stopMetrics()); err != nil {
			slog.Warn("Could not flush business metrics", "error", err)
		}
	}()
	ranking, err := configureRanking()
	if err != nil {
		return err
	}
	ranking.Observe = rankingObserver(signals)
	reconciler := reconciliation.NewWithRanking(pool, ranking)
	queue, err := reconciliation.NewQueue(pool, reconciler, workerEnabled)
	if err != nil {
		return err
	}

	var server *http.Server
	if apiEnabled {
		server, err = configureHTTP(pool, reconciler, queue, signals)
		if err != nil {
			return err
		}
	}
	if workerEnabled {
		workerContext, stopWorker := context.WithCancel(ctx)
		if err := startWorker(signals.Context(workerContext), queue); err != nil {
			stopWorker()
			return err
		}
		defer func() {
			stopWorker()
			<-queue.Stopped()
		}()
	}
	if server != nil {
		return serveHTTP(ctx, server)
	}
	<-ctx.Done()
	return nil
}

func componentEnabled(variable string) (bool, error) {
	enabled, err := strconv.ParseBool(cmp.Or(os.Getenv(variable), "true"))
	if err != nil {
		return false, fmt.Errorf("configure %s: %w", variable, err)
	}
	return enabled, nil
}

func startWorker(ctx context.Context, queue *reconciliation.Queue) error {
	if err := queue.Start(ctx); err != nil {
		return fmt.Errorf("start reconciliation worker: %w", err)
	}
	slog.Info("Reconciliation worker started")
	return nil
}

func configureHTTP(pool *pgxpool.Pool, reconciler *reconciliation.Service, queue *reconciliation.Queue, signals *telemetry.Runtime) (*http.Server, error) {
	key := []byte(os.Getenv("JWT_SECRET"))
	if len(key) == 0 {
		key = make([]byte, 32)
		rand.Read(key)
	} else if len(key) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	return &http.Server{
		Addr:              cmp.Or(os.Getenv("HTTP_ADDR"), ":8080"),
		Handler:           signals.HTTP(newHandler(pool, key, reconciler, queue)),
		ReadHeaderTimeout: 5 * time.Second,
	}, nil
}

func serveHTTP(ctx context.Context, server *http.Server) error {
	serveResult := make(chan error, 1)
	go func() {
		slog.Info("API listening", "address", server.Addr)
		serveResult <- server.ListenAndServe()
	}()

	select {
	case err := <-serveResult:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}

func newHandler(pool *pgxpool.Pool, key []byte, reconciler *reconciliation.Service, queue *reconciliation.Queue) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	api := httpapi.New(mux)
	accounts := identity.New(pool, key)
	accounts.RegisterHTTP(api)
	entries := catalog.New(pool)
	entries.RegisterHTTP(api, accounts)
	ingestion.New(pool, entries, queue).RegisterHTTP(api, accounts)
	queue.RegisterHTTP(api, accounts)
	reconciler.RegisterHTTP(api, accounts)
	return mux
}
