package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Decodes the body of an http request, and attempts to store it in a value of 
// type T.
//
// Example: 
// user, err := decode[User](r)
func decode[T any](r *http.Request) (T, error) {
	var v T
	
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("error when decoding JSON of type %w", err)
	}

	return v, nil

}
