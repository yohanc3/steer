/*
main.go

Author: Yohance Williams

This file contains the setup for a go routine that starts the http server, and another
which watches the http server for graceful shutdown.

Execution flow:

1. Set up dependencies (logger, database, etc.)
2. Create single instance of the http server
3. Initialize the http server in a separate go routine (non-blocking)
4. Spin up a shutdown watcher go routine for the http server
5. Wait for shutdown to complete (wg.Wait())
6. Program can exit safely

Good to know:
1. The program waits for the parent context Done() signal to trigger the graceful shutdown
2. Shutdown gives 10 seconds to the server to close all connections

*/

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

// All dependencies are given & used here when generating a Handler
func NewServer(logger *slog.Logger, db *sql.DB) http.Handler {

	var mux *http.ServeMux = http.NewServeMux()
	AddRoutes(mux, logger, db)

	var handler http.Handler = mux
	handler = AddCorsMiddleware(handler)

	return handler
}

func run(ctx context.Context) error {

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	err := godotenv.Load()

	if err != nil {
		panic(".env file not loaded correctly")
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	db, db_err := sql.Open("sqlite3", os.Getenv("DATABASE_URL"))
	if db_err != nil {
		panic(fmt.Sprintf("Unable to initialize database. Error: %s", db_err))
	}

	server := NewServer(logger, db)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: server,
	}

	// Go routine that starts the server
	go func() {
		fmt.Fprintf(os.Stdout, "Listening on port %s\n", httpServer.Addr)

		// Start listening if the returned error is not a bad
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Error listening and serving %s\n", err)
		}

	}()

	var wg sync.WaitGroup
	// Let our wait group know that we have 1 go routine still running
	wg.Add(1)

	// Set up shutdown watcher go routine
	go func() {
		defer wg.Done()

		// Hold until the parent context is interrupted (server is unmounted)
		<-ctx.Done()

		// Release resources after 10 seconds, for procesees that take longer to complete
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "Error shutting down http server listening at %s\n", err)
		}

	}()

	wg.Wait()
	return nil

}

func main() {

	parentCtx := context.Background()

	if err := run(parentCtx); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}

}
