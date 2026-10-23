package respond

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type ProblemDetails struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
}

func JSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		Problem(w, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}
func Problem(w http.ResponseWriter, status int) {
	data, _ := json.Marshal(ProblemDetails{Type: "about:blank", Title: http.StatusText(status), Status: status})
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}
func Decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(dst)
	if err == nil {
		var extra any
		err = decoder.Decode(&extra)
		if errors.Is(err, io.EOF) {
			return true
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		Problem(w, http.StatusRequestEntityTooLarge)
	} else {
		Problem(w, http.StatusBadRequest)
	}
	return false
}
