package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/Drumato/helmhound/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := cmd.New()

	if err := app.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
