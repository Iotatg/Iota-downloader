// Command iota-downloader runs the Iota Coder media downloader bot.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Iotatg/iota-downloader/internal/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := app.RunUntilSignal(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "iota-downloader stopped: %v\n", err)
		os.Exit(1)
	}
}
