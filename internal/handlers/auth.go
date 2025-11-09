package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/httpx"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/middleware"
	"github.com/google/uuid"
)


type User struct {
	ID           uuid.UUID
	Email        string
	Name         *string
	PasswordHash string
	Failed       int
	LockedUntil  *time.Time
	PwdRev       int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type RefreshToken struct {
	TokenHash string
	UserID    uuid.UUID
	DeviceID  string
	IssuedAt  time.Time
	ExpiresAt time.Time
	Revoked   bool
	RotatedTo *string
}


type UsersRepo interface {
	FindByEmail(ctx context.Context, email string) (*User, error)
	Insert(ctx context.Context, email, passwordHash string, name *string) (*User, error)
	SetFailed(ctx context.Context, id uuid.UUID, failed int, lockedUntil *time.Time) error
	ResetFailed(ctx context.Context, id uuid.UUID) error
	UpdatePassword(ctx context.Context, id uuid.UUID, newHash string, newRev int) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
}

type RefreshRepo interface {
	Insert(ctx context.Context, rt RefreshToken) error
	Get(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeChain(ctx context.Context, tokenHash string) error
	Rotate(ctx context.Context, oldHash, newHash string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

type JWTManager interface {
	IssueAccess(userID uuid.UUID, rev int) (string, error)
}

type RateLimiter interface {
	Allow(key string) bool 
}


type signupReq struct {
	Email    string  `json:"email"`
	Password string  `json:"password"`
	Name     *string `json:"name,omitempty"`
	DeviceID string  `json:"device_id"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	DeviceID string `json:"device_id"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
}

type passwordChangeReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type passwordResetReq struct {
	Email string `json:"email"`
}

type passwordResetConfirmReq struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}


type AuthHandlers struct {
	Users   UsersRepo
	RT      RefreshRepo
	JWT     JWTManager
	Hasher  PasswordHasher
	Limiter RateLimiter

	AccessTTL        time.Duration 
	RefreshTTL       time.Duration 
	MaxLoginFailures int           
	LockDuration     time.Duration 
}

// POST /auth/signup
func (h *AuthHandlers) Signup(w http.ResponseWriter, r *http.Request) {
	var in signupReq
	if err := jsonDecode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	in.Email = normEmail(in.Email)
	if in.Email == "" || in.Password == "" || in.DeviceID == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "email/password/device_id required")
		return
	}
	if u, _ := h.Users.FindByEmail(r.Context(), in.Email); u != nil {
		httpx.Error(w, http.StatusConflict, "already_exists", "user already exists")
		return
	}

	hash, err := h.Hasher.Hash(in.Password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "hash_error", "")
		return
	}
	u, err := h.Users.Insert(r.Context(), in.Email, hash, in.Name)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	access, err := h.JWT.IssueAccess(u.ID, u.PwdRev)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "jwt_error", "")
		return
	}

	rt := randomToken()
	rtHash := sha256Hex(rt)
	if err := h.RT.Insert(r.Context(), RefreshToken{
		TokenHash: rtHash,
		UserID:    u.ID,
		DeviceID:  in.DeviceID,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(h.RefreshTTL),
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"user": map[string]any{
			"id":    u.ID,
			"email": u.Email,
			"name":  u.Name,
		},
		"access_token":  access,
		"refresh_token": rt,
		"expires_in":    int(h.AccessTTL.Seconds()),
	})
}

// POST /auth/login
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var in loginReq
	if err := jsonDecode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	in.Email = normEmail(in.Email)
	if in.Email == "" || in.Password == "" || in.DeviceID == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "email/password/device_id required")
		return
	}
	if h.Limiter != nil && !h.Limiter.Allow("login:"+in.Email) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "")
		return
	}

	u, err := h.Users.FindByEmail(r.Context(), in.Email)
	if err != nil || u == nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	if u.LockedUntil != nil && time.Now().Before(*u.LockedUntil) {
		httpx.Error(w, http.StatusForbidden, "locked", "")
		return
	}

	if !h.Hasher.Compare(u.PasswordHash, in.Password) {
		failed := u.Failed + 1
		var lock *time.Time
		if h.MaxLoginFailures <= 0 {
			h.MaxLoginFailures = 5
		}
		if h.LockDuration <= 0 {
			h.LockDuration = 5 * time.Minute
		}
		if failed >= h.MaxLoginFailures {
			t := time.Now().Add(h.LockDuration)
			lock = &t
		}
		_ = h.Users.SetFailed(r.Context(), u.ID, failed, lock)
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	_ = h.Users.ResetFailed(r.Context(), u.ID)

	access, err := h.JWT.IssueAccess(u.ID, u.PwdRev)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "jwt_error", "")
		return
	}

	rt := randomToken()
	rtHash := sha256Hex(rt)
	if err := h.RT.Insert(r.Context(), RefreshToken{
		TokenHash: rtHash,
		UserID:    u.ID,
		DeviceID:  in.DeviceID,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(h.RefreshTTL),
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":    u.ID,
			"email": u.Email,
			"name":  u.Name,
		},
		"access_token":  access,
		"refresh_token": rt,
		"expires_in":    int(h.AccessTTL.Seconds()),
	})
}

// POST /auth/refresh
func (h *AuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var in refreshReq
	if err := jsonDecode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if in.RefreshToken == "" || in.DeviceID == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "refresh_token/device_id required")
		return
	}

	curHash := sha256Hex(in.RefreshToken)
	stored, err := h.RT.Get(r.Context(), curHash)
	if err != nil || stored == nil || stored.Revoked || time.Now().After(stored.ExpiresAt) || stored.DeviceID != in.DeviceID {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	u, err := h.Users.GetByID(r.Context(), stored.UserID)
	if err != nil || u == nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	access, err := h.JWT.IssueAccess(u.ID, u.PwdRev)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "jwt_error", "")
		return
	}

	// Sliding rotation
	newRT := randomToken()
	newHash := sha256Hex(newRT)
	if err := h.RT.Rotate(r.Context(), curHash, newHash); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": newRT,
		"expires_in":    int(h.AccessTTL.Seconds()),
	})
}

// POST /auth/logout
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	var in refreshReq
	if err := jsonDecode(r, &in); err != nil || in.RefreshToken == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json or empty token")
		return
	}
	_ = h.RT.RevokeChain(r.Context(), sha256Hex(in.RefreshToken))
	w.WriteHeader(http.StatusNoContent)
}

// POST /auth/password/change (Bearer)
func (h *AuthHandlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromCtx(r)
	if userID == uuid.Nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var in passwordChangeReq
	if err := jsonDecode(r, &in); err != nil || in.OldPassword == "" || in.NewPassword == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json or empty fields")
		return
	}

	u, err := h.Users.GetByID(r.Context(), userID)
	if err != nil || u == nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	if !h.Hasher.Compare(u.PasswordHash, in.OldPassword) {
		httpx.Error(w, http.StatusForbidden, "forbidden", "old password mismatch")
		return
	}

	newHash, err := h.Hasher.Hash(in.NewPassword)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "hash_error", "")
		return
	}

	if err := h.Users.UpdatePassword(r.Context(), userID, newHash, u.PwdRev+1); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	if h.RT != nil {
		_ = h.RT.RevokeAllForUser(r.Context(), userID)
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /auth/password/reset-request
func (h *AuthHandlers) ResetRequest(w http.ResponseWriter, r *http.Request) {
	var in passwordResetReq
	if err := jsonDecode(r, &in); err != nil || normEmail(in.Email) == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json or email")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /auth/password/reset-confirm
func (h *AuthHandlers) ResetConfirm(w http.ResponseWriter, r *http.Request) {
	var in passwordResetConfirmReq
	if err := jsonDecode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if normEmail(in.Email) == "" || in.Code == "" || in.NewPassword == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "email/code/new_password required")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}


func jsonDecode(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := jsonNewDecoder(r)
	return dec.Decode(v)
}

func jsonNewDecoder(r *http.Request) *json.Decoder {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec
}

func normEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		ts := time.Now().UnixNano()
		h := sha256.Sum256([]byte(strconv.FormatInt(ts, 10)))
		return base64.RawURLEncoding.EncodeToString(h[:])
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

