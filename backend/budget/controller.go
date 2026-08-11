package budget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const prototypeCookieName = "steer_prototype"

// Controller exposes the browser-review endpoints for the budget prototype.
type Controller struct{ Service Service }

// Register adds the complete prototype API beneath the existing same-origin API gateway.
func (controller Controller) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/prototype-budget", controller.view)
	mux.HandleFunc("POST /api/prototype-budget/chat", controller.chat)
	mux.HandleFunc("POST /api/prototype-budget/transactions", controller.addTransaction)
	mux.HandleFunc("POST /api/prototype-budget/unmatched/{id}/resolve", controller.resolveTransaction)
	mux.HandleFunc("DELETE /api/prototype-budget/budget", controller.deleteBudget)
	mux.HandleFunc("DELETE /api/prototype-budget", controller.reset)
}

func (controller Controller) view(w http.ResponseWriter, r *http.Request) {
	clientID := clientID(w, r)
	state, err := controller.Service.View(r.Context(), clientID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (controller Controller) chat(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Send a JSON object with a budget message."})
		return
	}
	clientID := clientID(w, r)
	result, err := controller.Service.SubmitBudget(r.Context(), clientID, request.Message)
	if err != nil {
		writeError(w, err)
		return
	}
	state, err := controller.Service.View(r.Context(), clientID)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusOK
	if result.Status == "invalid" {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, struct {
		Result ParseResult `json:"result"`
		State  State       `json:"state"`
	}{Result: result, State: state})
}

func (controller Controller) addTransaction(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Merchant         string `json:"merchant"`
		ProviderCategory string `json:"provider_category"`
		AmountCents      int64  `json:"amount_cents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Send a JSON transaction."})
		return
	}
	transaction, err := controller.Service.AddTransaction(r.Context(), clientID(w, r), Transaction{
		ID:               uuid.NewString(),
		Merchant:         strings.TrimSpace(request.Merchant),
		ProviderCategory: strings.TrimSpace(request.ProviderCategory),
		AmountCents:      request.AmountCents,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, transaction)
}

func (controller Controller) resolveTransaction(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Category string `json:"category"`
		Remember bool   `json:"remember"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Send a JSON category decision."})
		return
	}
	clientID := clientID(w, r)
	if err := controller.Service.ResolveUnmatched(r.Context(), clientID, r.PathValue("id"), request.Category, request.Remember); err != nil {
		writeError(w, err)
		return
	}
	state, err := controller.Service.View(r.Context(), clientID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (controller Controller) deleteBudget(w http.ResponseWriter, r *http.Request) {
	if err := controller.Service.DeleteBudget(r.Context(), clientID(w, r)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Budget deleted. Your test activity is still available."})
}

func (controller Controller) reset(w http.ResponseWriter, r *http.Request) {
	if err := controller.Service.Reset(r.Context(), clientID(w, r)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Prototype reset."})
}

func clientID(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(prototypeCookieName); err == nil && uuid.Validate(cookie.Value) == nil {
		return cookie.Value
	}
	id := uuid.NewString()
	http.SetCookie(w, &http.Cookie{
		Name:     prototypeCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 30,
	})
	return id
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	slog.Log(context.Background(), slog.LevelError, "budget prototype request failed", "error", err)
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrNoBudget):
		status = http.StatusConflict
	case errors.Is(err, ErrAIUnavailable):
		status = http.StatusServiceUnavailable
	case strings.Contains(err.Error(), "required"), strings.Contains(err.Error(), "choose one"):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": fmt.Sprintf("%v", err)})
}
