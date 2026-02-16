package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	applog "yohanc3/steer/logger"
)


func decode[T any](r *http.Request, logger *applog.Logger) (T, error) {
	var v T
	
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("error when decoding JSON of type %w", err)
	}

	return v, nil

}
