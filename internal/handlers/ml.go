package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)


type MLAdapter interface {
	Overview(ctx any, userID uuid.UUID) (any, error)
}

type MLHandlers struct {
	ML MLAdapter
}

// GET /api/v1/insights/overview
func (h *MLHandlers) Overview(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtxOrHeader(r)
	if userID == uuid.Nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if h.ML == nil {
		writeErr(w, http.StatusNotImplemented, "not_configured", "ml adapter is not configured")
		return
	}
	data, err := h.ML.Overview(r.Context(), userID)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "ml_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"overview": data})
}

// GET /api/v1/insights/categories?month=2025-11
func (h *MLHandlers) Categories(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtxOrHeader(r)
	if userID == uuid.Nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	_ = strings.TrimSpace(r.URL.Query().Get("month"))
	writeErr(w, http.StatusNotImplemented, "not_implemented", "")
}

var _ = json.NewDecoder
