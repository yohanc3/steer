package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"yohanc3/steer/teller"
)

// tellerConnectController validates Teller Connect completions and delegates
// verified enrollments to TellerService for persistence and initial syncing.
type tellerConnectController struct{ tellerService teller.TellerService }

// complete records a completed Teller Connect flow from the browser callback.
// The request must include the one-time session, signed enrollment, and token.
func (controller tellerConnectController) complete(w http.ResponseWriter, r *http.Request) {
	// Decode exactly the fields returned by Teller Connect's browser callback.
	var request struct {
		SessionToken string `json:"session_token"`
		AccessToken  string `json:"access_token"`
		Enrollment   struct {
			ID string `json:"id"`
		} `json:"enrollment"`
		User struct {
			ID string `json:"id"`
		} `json:"user"`
		Signatures []string `json:"signatures"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Verify and persist the completed enrollment before acknowledging the browser.
	if err := controller.tellerService.Complete(r.Context(), request.SessionToken, request.AccessToken, request.Enrollment.ID, request.User.ID, request.Signatures); err != nil {
		slog.Log(r.Context(), slog.LevelError, "complete teller connection", "error", err)
		http.Error(w, "unable to connect account", http.StatusBadRequest)
		return
	}

	// A successful completion deliberately returns no body to the frontend.
	w.WriteHeader(http.StatusNoContent)
}
