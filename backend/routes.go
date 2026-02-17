package main

import (
	"context"
	"database/sql"
	"net/http"
	applog "yohanc3/steer/logger"
)

type User struct {
	ID      string `json:"userID"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Picture string `json:"picture"`
}

// Adds all routes to a given multiplexer
func AddRoutes(mux *http.ServeMux, logger *applog.Logger, db *sql.DB) {
	mux.Handle("/user/{id}", CreateUser(logger, db))
}

func CreateUser(logger *applog.Logger, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		user, err := decode[User](r)

		if err != nil {
			logger.Error("Error when decoding user.")
			http.Error(w, "Error when decoding user.", http.StatusBadRequest)
			return
		}
		
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		tx, err := db.Begin()

		// Defer a rollback, so that it only runs if tx.Commit never occurs
		defer tx.Rollback()

		if err != nil {
			logger.Error("Initiaizing transaction failed.", "error", err.Error())
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			return
		}

		res, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO user (id, name, picture, email)
			VALUES (?, ?, ?, ?)
		`, user.ID, user.Name, user.Picture, user.Email)

		if err != nil {
			logger.Error("Inserting new user failed.", "error", err.Error())
			http.Error(w, "Creating user failed.", http.StatusInternalServerError)
			return
		}
		
		// Check if a new row was created. Log message depends on whether new user row was created or ignored
		rowsAffected, err := res.RowsAffected()

		if err != nil {
			logger.Error("Error retreiving affected rows from user insert.", "id", user.ID, "name", user.Name, "email", user.Email, "picture", user.Picture)
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			return
		}

		if rowsAffected <= 0 {
			logger.Info("201 - User already registered.", "id", user.ID, "name", user.Name, "email", user.Email, "picture", user.Picture)
		} else {
			logger.Info("201 - New User registered.", "id", user.ID, "name", user.Name, "email", user.Email, "picture", user.Picture)
		}

		if err = tx.Commit(); err != nil {
			logger.Error("Error commiting transaction into DB.", "id", user.ID, "name", user.Name, "email", user.Email, "picture", user.Picture)
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

	})
}
