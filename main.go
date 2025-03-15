package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/net/context"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)

	app, err := NewApp(os.Args, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create new app: %s\n", err)
		os.Exit(1)
	}

	if app == nil {
		os.Exit(0)
	}

	go func() {
		<-ctx.Done()
		// reset signal so server can be force killed with another signal (ctrl+c)
		stop()
	}()

	if err := app.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
