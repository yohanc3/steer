package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)


func decode[T any](r *http.Request, logger *slog.Logger) (T, error) {
	var v T
	
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("error when decoding JSON of type %w", err)
	}

	return v, nil

}