package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"

	"golang.org/x/sync/errgroup"
)

// getVCSRevision returns the git commit SHA if present else "devel".
func getVCSRevision() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}

	return "devel"
}

// App is our application instance.
type App struct {
	config  Config
	logger  *slog.Logger
	subber  *Subber
	ws      *WebServer
	version string
}

// NewApp creates an instance of our application, based on the supplied args and output locations.
func NewApp(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (*App, error) {
	logHandler := slog.NewTextHandler(stderr, nil).
		WithAttrs(
			[]slog.Attr{slog.String("version", getVCSRevision())},
		)
	logger := slog.New(logHandler)
	logger.Info("starting gosubs")

	fs := flag.NewFlagSet("gosubs", flag.ContinueOnError)
	configFile := fs.String("configFile", "config.json", "json file to read configuration from")
	showVersion := fs.Bool("version", false, "show version and exit")

	if err := fs.Parse(args[1:]); err != nil {
		return nil, fmt.Errorf("failed to parse args: %w", err)
	}

	if *showVersion {
		logger.Info("version requested, displaying and exiting", "version", getVCSRevision())
		return nil, nil
	}

	config := DefaultConfiguration()

	if _, err := os.Stat(*configFile); err == nil {
		f, err := os.Open(*configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		config, err = loadConfig(f)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}

		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("failed to close config file: %w", err)
		}

		logger.Info("loaded configuration from file", "file", *configFile)
	}

	subber := NewSubber(
		logger.WithGroup("subber"),
		config.Players,
	)

	ws, err := NewWebServer(
		logger.WithGroup("webserver"),
		subber,
	)
	if err != nil {
		return nil, err
	}

	app := &App{
		config:  config,
		logger:  logger,
		subber:  subber,
		ws:      ws,
		version: getVCSRevision(),
	}

	return app, nil
}

// Run starts the application and gracefully shuts down when the provided
// context is cancelled.
func (app *App) Run(ctx context.Context) error {
	app.logger.Info("running")

	cctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	g, gctx := errgroup.WithContext(cctx)

	// listen for cancellations
	g.Go(func() error {
		<-gctx.Done()

		if errors.Is(context.Cause(gctx), context.Canceled) {
			app.logger.Info("received OS signal to shutdown, use Ctrl+C again to force")
		}

		return nil
	})

	// start HTTP server
	g.Go(func() error {
		err := app.ws.Run(gctx)
		cancel(err)

		return err
	})

	err := g.Wait()
	if err != nil {
		return err
	}

	app.logger.Info("shutdown completed successfully")

	return nil
}
