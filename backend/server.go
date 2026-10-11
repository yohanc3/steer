package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	applog "yohanc3/steer/logger"
)

type HTTPMiddleware func(http.Handler) (http.Handler, error)
type PublicRouteRegistrar func(*http.ServeMux) error

type ServerDependencies struct {
	Logger               *applog.Logger
	DB                   *sql.DB
	TelegramService      TelegramConnectionService
	AuthMiddleware       HTTPMiddleware
	RegisterPublicRoutes PublicRouteRegistrar
	AllowedOrigins       []string
}

// NewServer assembles the HTTP handler from dependencies created by the caller.
// Production supplies Auth0 and Telegram integrations; tests can supply local fakes.
func NewServer(deps ServerDependencies) (http.Handler, error) {
	if deps.Logger == nil {
		return nil, errors.New("server logger is required")
	}
	if deps.DB == nil {
		return nil, errors.New("server database is required")
	}
	if deps.TelegramService == nil {
		return nil, errors.New("telegram connection service is required")
	}
	if deps.AuthMiddleware == nil {
		return nil, errors.New("auth middleware is required")
	}

	protectedMux := http.NewServeMux()
	AddRoutes(protectedMux, deps.Logger, deps.DB, deps.TelegramService)

	protectedHandler, err := deps.AuthMiddleware(protectedMux)
	if err != nil {
		return nil, err
	}

	rootMux := http.NewServeMux()
	rootMux.Handle("/api/", protectedHandler)
	rootMux.HandleFunc("GET /healthz", healthHandler(deps.DB))

	if deps.RegisterPublicRoutes != nil {
		if err := deps.RegisterPublicRoutes(rootMux); err != nil {
			return nil, err
		}
	}

	return AddCorsMiddleware(rootMux, deps.AllowedOrigins), nil
}

func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
