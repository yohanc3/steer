package main

import (
	"net/http"
	"yohanc3/steer/auth"
	"yohanc3/steer/config"
)

// Adds basic needed CORS permission headers to incoming requests from local port 5173.
// Returns an HTTP Handler, which carries the CORS permissions.
func AddCorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)

	})
}

func AddAccessTokenMiddleware(next http.Handler) (http.Handler, error) {

	jwtValidator, err := auth.NewValidator(config.Cfg.Auth0Domain, config.Cfg.Auth0Audience)
	if err != nil {
		return nil, err
	}

	middleware, err := auth.JWTMiddleware(jwtValidator)
	if err != nil {
		return nil, err
	}

 	jwtHandler := middleware.CheckJWT(next)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){

		if r.URL.Path == "/bot" {
			next.ServeHTTP(w, r)
			return
		}

		jwtHandler.ServeHTTP(w, r)
	}), nil
		
}
