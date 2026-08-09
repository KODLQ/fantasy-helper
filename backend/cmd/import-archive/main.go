package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"fantasy-helper/backend/internal/app"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: import-archive /path/to/manifest.json")
		os.Exit(2)
	}
	cfg, err := app.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	database, err := app.OpenDatabase(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer database.Close()
	repository := app.NewPostgresRepository(database, app.NewLogger(cfg.Environment))
	if err := repository.EnsureSchema(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	snapshot, err := app.ImportHistoricalArchive(ctx, os.Args[1], repository)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("imported season %d as %s (%s)\n", snapshot.SeasonID, snapshot.State, snapshot.ID)
}
