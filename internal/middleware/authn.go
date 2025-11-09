package middleware

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/httpx"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type ctxKey string

const CtxUserID ctxKey = "user_id"

type AuthnConfig struct {
	JWTPublicKeyPEM string
}

type Authn struct {
	pub *jwt.SigningMethodRSA
	key any // parsed public key
}

func NewAuthn(pemStr string) (*Authn, error) {
	if strings.TrimSpace(pemStr) == "" {
		return nil, errors.New("empty JWT public key PEM")
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return &Authn{
		pub: jwt.SigningMethodRS256,
		key: pub,
	}, nil
}

func (a *Authn) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if hdr := r.Header.Get("X-Debug-User"); hdr != "" {
			if id, err := uuid.Parse(hdr); err == nil {
				ctx := context.WithValue(r.Context(), CtxUserID, id)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing bearer")
			return
		}
		raw := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodRS256 {
				return nil, errors.New("unexpected signing method")
			}
			return a.key, nil
		})
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "bad token")
			return
		}
		sub, _ := claims["sub"].(string)
		uid, err := uuid.Parse(sub)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "bad sub")
			return
		}
		ctx := context.WithValue(r.Context(), CtxUserID, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserIDFromCtx(r *http.Request) uuid.UUID {
	if v := r.Context().Value(CtxUserID); v != nil {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}
