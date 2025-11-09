package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/banks"
	"github.com/google/uuid"
)


type ConsentManager interface {
	EnsureConsent(ctx any, userID uuid.UUID, bankCode string) (string, error)

	ListLinks(ctx any, userID uuid.UUID) ([]BankLinkDTO, error)

	RevokeConsent(ctx any, userID uuid.UUID, bankCode, consentID string) error
}

type BankLinkDTO struct {
	BankCode   string  `json:"bank_code"`
	ConsentID  string  `json:"consent_id"`
	Status     string  `json:"status"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
	CreatedAt  *string `json:"created_at,omitempty"`
	UpdatedAt  *string `json:"updated_at,omitempty"`
}

type BanksHandlers struct {
	Consents ConsentManager
}

// POST /api/v1/banks/{bank}/consents/request
func (h *BanksHandlers) RequestConsent(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtxOrHeader(r)
	if userID == uuid.Nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid path")
		return
	}
	bankCode := parts[2]

	cid, err := h.Consents.EnsureConsent(r.Context(), userID, bankCode)
	if err != nil {
		if err == banks.ErrConsentPending {
			writeErr(w, http.StatusAccepted, "consent_pending", "user interaction required")
			return
		}
		writeErr(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"consent_id": cid,
		"bank_code":  bankCode,
		"status":     "approved",
	})
}

// GET /api/v1/banks
func (h *BanksHandlers) ListLinks(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtxOrHeader(r)
	if userID == uuid.Nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	links, err := h.Consents.ListLinks(r.Context(), userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": links})
}

// DELETE /api/v1/banks/{bank}/consents/{cid}
func (h *BanksHandlers) RevokeConsent(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtxOrHeader(r)
	if userID == uuid.Nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid path")
		return
	}
	bankCode := parts[2]
	cid := parts[4]

	if err := h.Consents.RevokeConsent(r.Context(), userID, bankCode, cid); err != nil {
		writeErr(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var _ = json.NewDecoder
