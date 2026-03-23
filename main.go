package main

import (
	"log/slog"
	"os"

	"github.com/colinthatcher/podcast-stats/internal"
)

func main() {

	go internal.PeriodicallyFetchRSSFeed()

	// Run your server.
	if err := runServer(); err != nil {
		slog.Error("Failed to start server!", "details", err.Error())
		os.Exit(1)
	}
}
