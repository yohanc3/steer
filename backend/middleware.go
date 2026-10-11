package main

import (
	"net/http"
	"yohanc3/steer/auth"
	"yohanc3/steer/config"
)

// Adds CORS headers for explicitly configured web application origins.
func AddCorsMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowed[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
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

// Adds a JWT check to the supplied handler.
// See https://auth0.com/docs/quickstart/backend/golang/interactive
func AddAccessTokenMiddleware(next http.Handler) (http.Handler, error) {

	jwtValidator, err := auth.NewValidator(config.Cfg.Auth0Domain, config.Cfg.Auth0Audience)
	if err != nil {
		return nil, err
	}

	middleware, err := auth.JWTMiddleware(jwtValidator)
	if err != nil {
		return nil, err
	}

	return middleware.CheckJWT(next), nil

}
