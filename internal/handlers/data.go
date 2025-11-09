package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/banks"
	"github.com/google/uuid"
)

type BanksProvider interface {
	ClientByCode(code string) (banks.Client, bool)
}

type DataHandlers struct {
	Banks BanksProvider
}

// GET /api/v1/accounts?bank=vbank
func (h *DataHandlers) Accounts(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtxOrHeader(r)
	if userID == uuid.Nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	bankCode := strings.TrimSpace(r.URL.Query().Get("bank"))
	var out []banks.Account

	if bankCode != "" {
		cl, ok := h.Banks.ClientByCode(bankCode)
		if !ok {
			writeErr(w, http.StatusBadRequest, "bad_request", "unknown bank")
			return
		}
		accs, err := cl.Accounts(r.Context(), userID)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "upstream_error", err.Error())
			return
		}
		out = append(out, accs...)
	} else {
		for _, code := range []string{"vbank", "abank", "sbank"} {
			if cl, ok := h.Banks.ClientByCode(code); ok {
				accs, err := cl.Accounts(r.Context(), userID)
				if err != nil {
					writeErr(w, http.StatusBadGateway, "upstream_error", "bank="+code+": "+err.Error())
					return
				}
				out = append(out, accs...)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"accounts": out})
}

// GET /api/v1/transactions?bank=vbank&since=2025-11-01&limit=200
func (h *DataHandlers) Transactions(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtxOrHeader(r)
	if userID == uuid.Nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	q := r.URL.Query()
	bankCode := strings.TrimSpace(q.Get("bank"))
	if bankCode == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "bank is required")
		return
	}
	cl, ok := h.Banks.ClientByCode(bankCode)
	if !ok {
		writeErr(w, http.StatusBadRequest, "bad_request", "unknown bank")
		return
	}

	var since time.Time
	if s := strings.TrimSpace(q.Get("since")); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		} else if t2, err2 := time.Parse("2006-01-02", s); err2 == nil {
			since = t2
		} else {
			writeErr(w, http.StatusBadRequest, "bad_request", "invalid since format")
			return
		}
	} else {
		since = time.Now().Add(-30 * 24 * time.Hour)
	}

	limit := 500
	if l := strings.TrimSpace(q.Get("limit")); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 2000 {
			limit = v
		}
	}

	tx, err := cl.Transactions(r.Context(), userID, since, limit)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": tx})
}


type ctxUserIDKey struct{}

func userIDFromCtxOrHeader(r *http.Request) uuid.UUID {
	if v := r.Context().Value(ctxUserIDKey{}); v != nil {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	if s := strings.TrimSpace(r.Header.Get("X-Debug-User")); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			return id
		}
	}
	return uuid.Nil
}

func WithUserID(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := context.WithValue(r.Context(), ctxUserIDKey{}, userID)
	return r.WithContext(ctx)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, kind, msg string) {
	writeJSON(w, code, map[string]any{"error": kind, "message": msg})
}
