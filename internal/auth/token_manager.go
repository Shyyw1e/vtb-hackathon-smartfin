// internal/auth/token_manager.go
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/cache"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/config"
)


type tokenManager struct {
	httpc     *http.Client
	banks     map[string]config.BankConfig 
	cache     cache.RedisStore
	group     singleflight.Group
	now       func() time.Time
	keyPrefix string
}

type bankTokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`  
	ExpiresAt   string `json:"expires_at,omitempty"`  
}

type tokenCache struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewTokenManager(httpc *http.Client, banks map[string]config.BankConfig, cache cache.RedisStore) TokenProvider {
	if httpc == nil {
		httpc = &http.Client{Timeout: 4 * time.Second}
	}
	return &tokenManager{
		httpc:     httpc,
		banks:     banks,
		cache:     cache,
		now:       time.Now,
		keyPrefix: "sf:bank:token",
	}
}

func (m *tokenManager) GetToken(ctx context.Context, bank BankCode) (string, error) {
	if tok := m.getFromCache(ctx, bank); tok != "" {
		return tok, nil
	}
	sfKey := "refresh:" + string(bank)
	v, err, _ := m.group.Do(sfKey, func() (any, error) {
		return m.fetchAndCache(ctx, bank)
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (m *tokenManager) ForceRefresh(ctx context.Context, bank BankCode) (string, error) {
	sfKey := "force:" + string(bank)
	v, err, _ := m.group.Do(sfKey, func() (any, error) {
		return m.fetchAndCache(ctx, bank)
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}


func (m *tokenManager) bankCfg(bank BankCode) (config.BankConfig, bool) {
	b, ok := m.banks[string(bank)]
	return b, ok
}

func (m *tokenManager) redisKey(bank BankCode, clientID string) string {
	return fmt.Sprintf("%s:%s:%s", m.keyPrefix, bank, clientID)
}

func (m *tokenManager) getFromCache(ctx context.Context, bank BankCode) string {
	bcfg, ok := m.bankCfg(bank)
	if !ok || bcfg.ClientID == "" {
		return ""
	}
	key := m.redisKey(bank, bcfg.ClientID)

	var tc tokenCache
	found, err := m.cache.GetJSON(ctx, key, &tc)
	if err != nil || !found {
		return ""
	}
	if tc.ExpiresAt.Before(m.now()) {
		_ = m.cache.Del(ctx, key) 
		return ""
	}
	return tc.Token
}

func (m *tokenManager) fetchAndCache(ctx context.Context, bank BankCode) (string, error) {
	bcfg, ok := m.bankCfg(bank)
	if !ok {
		return "", fmt.Errorf("token_manager: unknown bank %q", bank)
	}
	if bcfg.ClientID == "" || bcfg.ClientSecret == "" {
		return "", fmt.Errorf("token_manager: bank %q has no client creds (check env BANKS_%s_CLIENT_ID/SECRET)",
			bank, strings.ToUpper(string(bank)))
	}

	url := strings.TrimRight(bcfg.BaseURL, "/") + "/auth/bank-token"

	body := map[string]string{
		"client_id":     bcfg.ClientID,
		"client_secret": bcfg.ClientSecret,
	}
	jb, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("token_manager: marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jb))
	if err != nil {
		return "", fmt.Errorf("token_manager: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("token_manager: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return "", fmt.Errorf("token_manager: bank-token status=%d body=%s", resp.StatusCode, string(b))
	}

	var br bankTokenResp
	if err := json.NewDecoder(resp.Body).Decode(&br); err != nil {
		return "", fmt.Errorf("token_manager: decode resp: %w", err)
	}
	if br.AccessToken == "" {
		return "", fmt.Errorf("token_manager: empty access_token")
	}

	now := m.now()
	var expAt time.Time
	switch {
	case br.ExpiresIn > 0:
		expAt = now.Add(time.Duration(br.ExpiresIn) * time.Second)
	case br.ExpiresAt != "":
		if t, err := time.Parse(time.RFC3339, br.ExpiresAt); err == nil {
			expAt = t
		} else {
			expAt = now.Add(1 * time.Hour)
		}
	default:
		expAt = now.Add(1 * time.Hour)
	}

	safeExp := expAt.Add(-2 * time.Minute)
	if safeExp.Before(now.Add(15 * time.Second)) {
		safeExp = now.Add(15 * time.Second)
	}
	ttl := time.Until(safeExp)

	key := m.redisKey(bank, bcfg.ClientID)
	tc := tokenCache{Token: br.AccessToken, ExpiresAt: expAt}

	if err := m.cache.SetJSON(ctx, key, &tc, ttl); err != nil {
		return "", fmt.Errorf("token_manager: cache set: %w", err)
	}

	return br.AccessToken, nil
}
