package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/config"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/export"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/httpapi"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = cmdServe()
	case "migrate":
		err = cmdMigrate()
	case "healthcheck":
		err = cmdHealthcheck()
	case "config":
		err = cmdConfig(os.Args[2:])
	case "audit":
		err = cmdAudit(os.Args[2:])
	case "export-sample":
		err = cmdExport(os.Args[2:])
	case "version":
		fmt.Printf("%s %s\n", version, commit)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "reisekosten: %s\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: reisekosten <command>
  serve
  migrate
  healthcheck
  config check
  audit verify
  export-sample [--out file]
  version
`)
}

func cmdServe() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := newLogger(cfg)
	slog.SetDefault(log)
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return err
	}
	if cfg.StorageBackend == "local" {
		if err := os.MkdirAll(cfg.StorageLocalPath, 0o755); err != nil {
			return err
		}
	}
	if cfg.StorageBackend == "s3" && cfg.S3.AllowNonEU {
		log.Warn("S3_ALLOW_NON_EU is set; object storage may be outside the EU/EEA")
	}
	pw, err := auth.NewPasswords(auth.Params{
		Memory: cfg.Argon2MemoryKiB, Time: cfg.Argon2Time, Threads: cfg.Argon2Threads, KeyLen: 32,
	})
	if err != nil {
		return err
	}
	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	if err := httpapi.Bootstrap(ctx, st, cfg, pw, log); err != nil {
		return err
	}
	app, err := httpapi.New(cfg, st, pw, log)
	if err != nil {
		return err
	}
	app.Version = version
	app.Commit = commit
	jobCtx, stopJobs := context.WithCancel(context.Background())
	defer stopJobs()
	app.StartJobs(jobCtx)
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.ListenAddr, "version", version)
	sig, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-sig.Done():
		stopJobs()
	}
	shut, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shut)
}

func cmdMigrate() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return err
	}
	st, err := store.Open(context.Background(), cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	v, err := st.DBVersion(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("schema version %d\n", v)
	return nil
}

func cmdHealthcheck() error {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/healthz", nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz status %d", res.StatusCode)
	}
	return nil
}

func cmdConfig(args []string) error {
	if len(args) != 1 || args[0] != "check" {
		return errors.New("usage: reisekosten config check")
	}
	if _, err := config.Load(); err != nil {
		return err
	}
	fmt.Println("configuration ok")
	return nil
}

func cmdAudit(args []string) error {
	if len(args) != 1 || args[0] != "verify" {
		return errors.New("usage: reisekosten audit verify")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := store.Open(context.Background(), cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	if err := st.VerifyAudit(context.Background()); err != nil {
		return err
	}
	fmt.Println("audit chain ok")
	return nil
}

func cmdExport(args []string) error {
	out := "/tmp/sample.pdf"
	fs := flag.NewFlagSet("export-sample", flag.ContinueOnError)
	fs.StringVar(&out, "out", out, "destination PDF")
	if err := fs.Parse(args); err != nil {
		return err
	}
	typst := os.Getenv("TYPST_PATH")
	if typst == "" {
		typst = "/usr/local/bin/typst"
	}
	if err := export.Sample(typst, out); err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func newLogger(cfg config.Config) *slog.Logger {
	var lvl slog.Level
	switch cfg.LogLevel {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}
