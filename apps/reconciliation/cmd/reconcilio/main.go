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
	"github.com/kmmenna/reconcilio/apps/reconciliation/migrations"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	command := "serve"
	if len(os.Args) == 2 {
		command = os.Args[1]
	}
	if len(os.Args) > 2 {
		slog.Error("Expected one command: serve, migrate, or bootstrap-admin")
		os.Exit(1)
	}
	if err := run(ctx, command); err != nil {
		slog.Error("Reconcilio command failed", "command", command, "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, command string) error {
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
		return serve(ctx, pool)
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

func serve(ctx context.Context, pool *pgxpool.Pool) error {
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
	reconciler := reconciliation.New(pool)
	queue, err := reconciliation.NewQueue(pool, reconciler, workerEnabled)
	if err != nil {
		return err
	}

	var server *http.Server
	if apiEnabled {
		server, err = configureHTTP(pool, reconciler, queue)
		if err != nil {
			return err
		}
	}
	if workerEnabled {
		workerContext, stopWorker := context.WithCancel(ctx)
		if err := startWorker(workerContext, queue); err != nil {
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

func configureHTTP(pool *pgxpool.Pool, reconciler *reconciliation.Service, queue *reconciliation.Queue) (*http.Server, error) {
	key := []byte(os.Getenv("JWT_SECRET"))
	if len(key) == 0 {
		key = make([]byte, 32)
		rand.Read(key)
	} else if len(key) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	return &http.Server{
		Addr:              cmp.Or(os.Getenv("HTTP_ADDR"), ":8080"),
		Handler:           newHandler(pool, key, reconciler, queue),
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

func newHandler(pool *pgxpool.Pool, key []byte, reconciler *reconciliation.Service, queue *reconciliation.Queue) http.Handler {
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
