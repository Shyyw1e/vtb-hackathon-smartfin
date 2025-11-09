package httpx

import (
	"encoding/json"
	"net/http"
)

type ErrPayload struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func Error(w http.ResponseWriter, status int, code, detail string) {
	JSON(w, status, ErrPayload{Code: code, Detail: detail})
}
