package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
)

type User struct {
	Id			uint32	`json:"id"`
	Name		string	`json:"name"`
	Email		string	`json:"email"`
	UserGuid	string	`json:"user_guid"`
	Picture		string	`json:"picture"`
}

func addRoutes(mux *http.ServeMux, logger *slog.Logger, db *sql.DB) {
	mux.Handle("/", create_user(logger, db))
}

func create_user(logger *slog.Logger, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){

		user, err := decode[User](r, logger)

		if err != nil {
			logger.Error("Error when decoding user.")
		}

		fmt.Println("response json: ", user)

	})
}