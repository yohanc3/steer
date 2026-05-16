package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	applog "yohanc3/steer/logger"
	telegram "yohanc3/steer/telegrambot"
)

type User struct {
	ID                    string `json:"ID"`
	Name                  string `json:"name"`
	Email                 string `json:"email"`
	Picture               string `json:"picture"`
	IsConnectedToTelegram bool   `json:"isConnectedToTelegram"`
}

type appError struct {
	Err         error
	Message     string
	MessageArgs []string
	Code        int
}

type appHandler func(w http.ResponseWriter, r *http.Request) *appError

func (err *appError) Error() string { return err.Err.Error() }

// Wraps a route handler to log any returned error 
func wrapHandler(logger *applog.Logger, handler appHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// using a custom error allows us to handle any error returned
		if appErr := handler(w, r); appErr != nil {
			http.Error(w, appErr.Message, appErr.Code)
			logger.Debug(appErr.Err.Error(), appErr.MessageArgs)
		}
	})
}

// Adds all routes to a given multiplexer
func AddRoutes(mux *http.ServeMux, logger *applog.Logger, db *sql.DB, telegramService *telegram.TelegramService) {

	mux.Handle("POST /api/user", wrapHandler(logger, CreateUser(logger, db)))
	mux.Handle("GET /api/user/{user_id}/telegram/connection-code", wrapHandler(logger, GetTelegramConnectionCode(logger, db, telegramService)))
}

// Frontend requests code to send to the Bot, which is linked in the backend to
// their user, so when the Bot receives it, it sends it along with the conversation
// id, which gets linked to the user so we know it's them.
func GetTelegramConnectionCode(logger *applog.Logger, db *sql.DB, telegramService *telegram.TelegramService) appHandler {
	return func(w http.ResponseWriter, r *http.Request) *appError {

		user_id := r.PathValue("user_id")

		if user_id == "" {
			error := "Error when getting 'user_id' from path value."
			return &appError{fmt.Errorf(error), "Bad Request.", []string{}, http.StatusBadRequest}
		}

		otp, err := telegramService.GetConnectionCode(r.Context(), user_id)

		if err != nil {
			return &appError{fmt.Errorf("error when sending user telegram connection code %w"), "Internal Server Error.", []string{"user_id", user_id}, http.StatusInternalServerError}
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(otp)
		logger.Debug("successfully sent telegram connection code", "user_id", user_id)

		return nil

	}
}

// Creates new user if doesn't exist. Returns populated user object.
func CreateUser(logger *applog.Logger, db *sql.DB) appHandler {
	return func(w http.ResponseWriter, r *http.Request) *appError {

		user, err := decode[User](r)
		fmt.Println("user id gotten is:" + user.ID)

		if err != nil {
			return &appError{err, "Bad Request.", nil, http.StatusBadRequest}
		}

		dbRow := db.QueryRowContext(r.Context(),
			`INSERT INTO user (id, name, email, picture)
			 VALUES (?, ?, ?, ?)
		     ON CONFLICT(id) DO UPDATE SET id=id RETURNING (conversation_id IS NOT NULL) AS isConnectedToTelegram;`,
			user.ID, user.Name, user.Email, user.Picture)

		err = dbRow.Scan(&user.IsConnectedToTelegram)

		if err != nil {
			return &appError{err, "Internal Server Error.", []string{"user_id", user.ID}, http.StatusInternalServerError}
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(user)

		return nil
	}
}
