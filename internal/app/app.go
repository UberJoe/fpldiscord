// Package app owns the boot sequence and graceful shutdown for the single
// fpldiscord process. cmd/fpldiscord/main.go is a thin shell over New + Run.
//
// Ticket 01 covers: build the logger, log the effective config, build snapshot
// #1 with a <=30s budget (non-zero exit if it never succeeds), start the HTTP
// server, open the Discord gateway (BulkOverwrite runs on READY inside bot),
// then block until the context is cancelled and shut down HTTP -> Discord.
// Ticket 02 adds the fpl refresher goroutine, started in Run and stopped when
// the run context is cancelled. Ticket 10 opens the SQLite bet store and runs
// its migrations before Discord connects; a failed migration is a non-zero exit.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/UberJoe/fpldiscord/internal/bot"
	"github.com/UberJoe/fpldiscord/internal/config"
	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/UberJoe/fpldiscord/internal/store"
	"github.com/UberJoe/fpldiscord/internal/web"
)

// snapshotOneBudget bounds the first snapshot build at boot. Exceeding it is a
// non-zero exit so fly restarts the machine.
const snapshotOneBudget = 30 * time.Second

// shutdownGrace bounds the graceful HTTP shutdown.
const shutdownGrace = 10 * time.Second

// App is the wired-up process: HTTP server, Discord bot, fpl snapshot refresher,
// SQLite bet store.
type App struct {
	log        *slog.Logger
	cfg        config.Config
	httpServer *http.Server
	bot        *bot.Bot
	refresher  *fpl.Refresher
	betStore   *store.Store
}

// New runs boot steps 1–3: logger + effective-config log, then snapshot #1
// within snapshotOneBudget. The context lets a SIGINT/SIGTERM during the boot
// snapshot abort the wait. It returns an error (mapped to a non-zero exit by
// main) if the snapshot never builds.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogSlogLevel()}))
	cfg.LogEffective(log)

	// Open the bet DB and run migrations before Discord connects: a bad
	// migration must stop the release, not half-serve.
	betStore, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	log.Info("bet store opened", "path", cfg.DBPath)

	// Anything that fails after this point must close the DB it just opened;
	// clearing ok on the way to a successful return disarms the cleanup.
	ok := false
	defer func() {
		if !ok {
			betStore.Close()
		}
	}()

	snap := fpl.NewStore()
	client := fpl.NewClient(cfg.LeagueID)
	refresher := fpl.NewRefresher(client, snap, log)
	if err := refresher.Bootstrap(ctx, snapshotOneBudget); err != nil {
		return nil, err
	}

	b, err := bot.New(cfg, log, snap, betStore)
	if err != nil {
		return nil, err
	}

	// web resolves bettor Discord ids to display names through the bot's
	// member-name cache, injected here so web never imports bot.
	srv := web.New(log, snap).WithBet(betStore, b, cfg.Season)
	httpServer := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ok = true
	return &App{
		log:        log,
		cfg:        cfg,
		httpServer: httpServer,
		bot:        b,
		refresher:  refresher,
		betStore:   betStore,
	}, nil
}

// Run runs boot steps 4–7: start HTTP, start the fpl refresher, open Discord,
// block on ctx, then shut down HTTP -> Discord. The refresher goroutine stops
// on its own when refreshCtx is cancelled during shutdown.
func (a *App) Run(ctx context.Context) error {
	refreshCtx, stopRefresher := context.WithCancel(context.Background())
	defer stopRefresher()

	serverErr := make(chan error, 1)
	go func() {
		a.log.Info("http listening", "addr", a.httpServer.Addr)
		err := a.httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	refresherDone := make(chan struct{})
	go func() {
		defer close(refresherDone)
		a.refresher.Run(refreshCtx)
	}()
	a.log.Info("fpl refresher started", "interval", fpl.RefreshInterval.String())

	if err := a.bot.Open(); err != nil {
		stopRefresher()
		<-refresherDone
		a.shutdownHTTP()
		a.betStore.Close()
		return err
	}
	a.log.Info("discord gateway open")

	var runErr error
	select {
	case <-ctx.Done():
		a.log.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			a.log.Error("http server stopped with error", "err", err)
			runErr = err
		}
	}

	a.shutdownHTTP()
	stopRefresher()
	<-refresherDone
	a.log.Info("fpl refresher stopped")
	if err := a.bot.Close(); err != nil {
		a.log.Error("discord close failed", "err", err)
	} else {
		a.log.Info("discord gateway closed")
	}
	if err := a.betStore.Close(); err != nil {
		a.log.Error("bet store close failed", "err", err)
	} else {
		a.log.Info("bet store closed")
	}
	return runErr
}

func (a *App) shutdownHTTP() {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := a.httpServer.Shutdown(ctx); err != nil {
		a.log.Error("http shutdown failed", "err", err)
		return
	}
	a.log.Info("http server stopped")
}
