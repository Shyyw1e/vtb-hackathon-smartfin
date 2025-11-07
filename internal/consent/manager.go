package consent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/auth"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/cache"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/config"
	"golang.org/x/sync/singleflight"

	"github.com/google/uuid"
)

var (
	ErrConsentPending = errors.New("consent pending, user interaction required")
	ErrConsentMissing = errors.New("consent missing")
)


type Link struct {
	UserID            uuid.UUID
	Bank              auth.BankCode
	ConsentID         string
	ConsentStatus     string       
	ConsentExpiresAt  *time.Time   
}

type Repo interface {
	GetLink(ctx context.Context, userID uuid.UUID, bank auth.BankCode) (*Link, error)
	UpsertConsent(ctx context.Context, userID uuid.UUID, bank auth.BankCode, id string, status string, expiresAt *time.Time) error
	UpdateConsentStatus(ctx context.Context, userID uuid.UUID, bank auth.BankCode, status string, expiresAt *time.Time) error
	ClearConsent(ctx context.Context, userID uuid.UUID, bank auth.BankCode) error
}


type Manager struct {
	httpc        *http.Client
	cfg          *config.Config
	tp           auth.TokenProvider
	repo         Repo
	redis        cache.RedisStore // может быть nil
	teamID       string
	pollAttempts int           // по умолчанию 5
	pollInterval time.Duration // по умолчанию 2s
	group        singleflight.Group
}

type Option func(*Manager)

func WithRedis(c cache.RedisStore) Option {
	return func(m *Manager) { m.redis = c }
}
func WithPoll(attempts int, interval time.Duration) Option {
	return func(m *Manager) {
		if attempts > 0 {
			m.pollAttempts = attempts
		}
		if interval > 0 {
			m.pollInterval = interval
		}
	}
}

func New(httpc *http.Client, cfg *config.Config, tp auth.TokenProvider, repo Repo, opts ...Option) (*Manager, error) {
	if httpc == nil {
		return nil, fmt.Errorf("http client is nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if tp == nil {
		return nil, fmt.Errorf("token provider is nil")
	}
	if repo == nil {
		return nil, fmt.Errorf("repo is nil")
	}
	if strings.TrimSpace(cfg.App.TeamID) == "" {
		return nil, fmt.Errorf("app.team_id is empty (needed for X-Requesting-Bank)")
	}
	m := &Manager{
		httpc:        httpc,
		cfg:          cfg,
		tp:           tp,
		repo:         repo,
		teamID:       cfg.App.TeamID,
		pollAttempts: 5,
		pollInterval: 2 * time.Second,
	}
	for _, o := range opts {
		o(m)
	}
	return m, nil
}

func (m *Manager) EnsureConsent(ctx context.Context, userID uuid.UUID, bank auth.BankCode) (string, error) {
	if id, ok := m.getConsentFromCache(ctx, userID, bank); ok {
		return id, nil
	}

	key := fmt.Sprintf("consent:%s:%s", userID.String(), string(bank))
	v, err, _ := m.group.Do(key, func() (any, error) {

		if id, ok := m.getConsentFromCache(ctx, userID, bank); ok {
			return id, nil
		}

		link, _ := m.repo.GetLink(ctx, userID, bank)
		if link != nil && link.ConsentID != "" {
			if isActive(link.ConsentStatus, link.ConsentExpiresAt) {
				m.setConsentToCache(ctx, userID, bank, link.ConsentID, link.ConsentExpiresAt)
				return link.ConsentID, nil
			}
			if strings.EqualFold(link.ConsentStatus, "pending") {
				id, st, exp, err := m.pollStatus(ctx, bank, link.ConsentID)
				if err == nil && strings.EqualFold(st, "active") {
					_ = m.repo.UpdateConsentStatus(ctx, userID, bank, "active", exp)
					m.setConsentToCache(ctx, userID, bank, id, exp)
					return id, nil
				}
				return "", ErrConsentPending
			}
		}

		id, st, exp, err := m.createConsent(ctx, bank, userID)
		if err != nil {
			return "", err
		}
		switch strings.ToLower(st) {
		case "approved", "active":
			_ = m.repo.UpsertConsent(ctx, userID, bank, id, "active", exp)
			m.setConsentToCache(ctx, userID, bank, id, exp)
			return id, nil
		case "pending":
			_ = m.repo.UpsertConsent(ctx, userID, bank, id, "pending", exp)
			id2, st2, exp2, err := m.pollStatus(ctx, bank, id)
			if err == nil && strings.EqualFold(st2, "active") {
				_ = m.repo.UpdateConsentStatus(ctx, userID, bank, "active", exp2)
				m.setConsentToCache(ctx, userID, bank, id2, exp2)
				return id2, nil
			}
			return "", ErrConsentPending
		default:
			_ = m.repo.UpsertConsent(ctx, userID, bank, id, st, exp)
			return "", ErrConsentMissing
		}
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (m *Manager) RevokeConsent(ctx context.Context, userID uuid.UUID, bank auth.BankCode) error {
	link, err := m.repo.GetLink(ctx, userID, bank)
	if err != nil || link == nil || link.ConsentID == "" {
		return nil 
	}
	base := m.bankBaseURL(bank)
	url := fmt.Sprintf("%s/account-consents/%s", base, url.PathEscape(link.ConsentID))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requesting-Bank", m.teamID)
	if tok, terr := m.tp.GetToken(ctx, bank); terr == nil {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := m.httpc.Do(req)
	if err != nil {
		return err
	}
	defer drainAndClose(resp.Body)

	switch resp.StatusCode {
	case http.StatusNoContent: 
		_ = m.repo.UpdateConsentStatus(ctx, userID, bank, "revoked", nil)
		m.delConsentFromCache(ctx, userID, bank)
		return nil
	case http.StatusNotFound:
		_ = m.repo.ClearConsent(ctx, userID, bank)
		m.delConsentFromCache(ctx, userID, bank)
		return nil
	default:
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("revoke consent: status=%d body=%s", resp.StatusCode, string(b))
	}
}


func (m *Manager) bankBaseURL(bank auth.BankCode) string {
	b, ok := m.cfg.Banks[string(bank)]
	if !ok || strings.TrimSpace(b.BaseURL) == "" {
		return ""
	}
	trim := strings.TrimRight(b.BaseURL, "/")
	return trim
}

func (m *Manager) createConsent(ctx context.Context, bank auth.BankCode, userID uuid.UUID) (id string, status string, expires *time.Time, err error) {
	base := m.bankBaseURL(bank)
	if base == "" {
		return "", "", nil, fmt.Errorf("bank base url is empty for %s", bank)
	}
	endpoint := fmt.Sprintf("%s/account-consents/request", base)

	payload := map[string]any{
		"client_id":            userID.String(), // если песочница требует client_id — можно подставлять своего юзера
		"permissions":          []string{"ReadAccountsDetail", "ReadBalances", "ReadTransactionsDetail"},
		"reason":               "Data aggregation for SmartFin",
		"requesting_bank":      m.teamID,
		"requesting_bank_name": "SmartFin App",
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", "", nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requesting-Bank", m.teamID)
	if tok, terr := m.tp.GetToken(ctx, bank); terr == nil {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := m.httpc.Do(req)
	if err != nil {
		return "", "", nil, err
	}
	defer drainAndClose(resp.Body)

	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		return "", "", nil, fmt.Errorf("create consent: status=%d body=%s", resp.StatusCode, string(b))
	}

	// гибкий парсер: песочница может вернуть разные схемы
	var (
		obj1 struct {
			ConsentID   string  `json:"consent_id"`
			Status      string  `json:"status"`
			Expires     *string `json:"expirationDateTime"`
		}
		obj2 struct {
			Data struct {
				ConsentID  string  `json:"consentId"`
				Status     string  `json:"status"`
				Expires    *string `json:"expirationDateTime"`
			} `json:"data"`
		}
		plain string
	)
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &obj1)
	if obj1.ConsentID != "" {
		return obj1.ConsentID, normalizeStatus(obj1.Status), parseTimePtr(obj1.Expires), nil
	}
	_ = json.Unmarshal(raw, &obj2)
	if obj2.Data.ConsentID != "" {
		return obj2.Data.ConsentID, normalizeStatus(obj2.Data.Status), parseTimePtr(obj2.Data.Expires), nil
	}
	_ = json.Unmarshal(raw, &plain)
	if strings.HasPrefix(plain, "consent-") {
		// неизвестный статус — пометим как pending
		return plain, "pending", nil, nil
	}
	// как запасной путь — распарсим map и попробуем вытащить поля
	var anyMap map[string]any
	if err := json.Unmarshal(raw, &anyMap); err == nil {
		if v, ok := anyMap["consent_id"].(string); ok && v != "" {
			st, _ := anyMap["status"].(string)
			return v, normalizeStatus(st), nil, nil
		}
	}
	return "", "", nil, fmt.Errorf("unexpected consent response: %s", string(raw))
}

func (m *Manager) pollStatus(ctx context.Context, bank auth.BankCode, consentID string) (id string, status string, expires *time.Time, err error) {
	base := m.bankBaseURL(bank)
	if base == "" {
		return "", "", nil, fmt.Errorf("bank base url is empty for %s", bank)
	}
	endpoint := fmt.Sprintf("%s/account-consents/%s", base, url.PathEscape(consentID))
	attempts := m.pollAttempts
	if attempts <= 0 {
		attempts = 5
	}
	interval := m.pollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	for i := 0; i < attempts; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", "", nil, err
		}
		req.Header.Set("Accept", "application/json")
		// X-Requesting-Bank опционален
		req.Header.Set("X-Requesting-Bank", m.teamID)
		// Авторизация не обязательна, но не мешает
		if tok, terr := m.tp.GetToken(ctx, bank); terr == nil {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := m.httpc.Do(req)
		if err != nil {
			// считаем временной проблемой, ждём и дальше
			select {
			case <-ctx.Done():
				return "", "", nil, ctx.Err()
			case <-time.After(interval):
				continue
			}
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			// локальная запись битая — пусть `EnsureConsent` создаст заново
			return "", "", nil, ErrConsentMissing
		}
		if resp.StatusCode/100 != 2 {
			// временная ошибка — подождём и продолжим
			select {
			case <-ctx.Done():
				return "", "", nil, ctx.Err()
			case <-time.After(interval):
				continue
			}
		}

		var obj struct {
			Data struct {
				ConsentID  string  `json:"consentId"`
				Status     string  `json:"status"`
				Expires    *string `json:"expirationDateTime"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &obj); err == nil && obj.Data.ConsentID != "" {
			return obj.Data.ConsentID, normalizeStatus(obj.Data.Status), parseTimePtr(obj.Data.Expires), nil
		}
		// неизвестный ответ — попробуем ещё раз
		select {
		case <-ctx.Done():
			return "", "", nil, ctx.Err()
		case <-time.After(interval):
			continue
		}
	}
	// не успели получить approved — остаётся pending
	return consentID, "pending", nil, ErrConsentPending
}

// ---- cache helpers ----

func (m *Manager) cacheKey(userID uuid.UUID, bank auth.BankCode) string {
	return fmt.Sprintf("consent:%s:%s", userID.String(), string(bank))
}

func (m *Manager) getConsentFromCache(ctx context.Context, userID uuid.UUID, bank auth.BankCode) (string, bool) {
	if m.redis == nil {
		return "", false
	}
	var v struct {
		ID  string     `json:"id"`
		St  string     `json:"st"`
		Exp *time.Time `json:"exp,omitempty"`
	}
	ok, err := m.redis.GetJSON(ctx, m.cacheKey(userID, bank), &v)
	if err != nil || !ok {
		return "", false
	}
	if isActive(v.St, v.Exp) && v.ID != "" {
		return v.ID, true
	}
	return "", false
}

func (m *Manager) setConsentToCache(ctx context.Context, userID uuid.UUID, bank auth.BankCode, id string, expires *time.Time) {
	if m.redis == nil || id == "" {
		return
	}
	ttl := time.Hour
	if expires != nil {
		d := time.Until(*expires) - 2*time.Minute
		if d > 10*time.Second {
			ttl = d
		}
	}
	_ = m.redis.SetJSON(ctx, m.cacheKey(userID, bank), map[string]any{
		"id": id, "st": "active", "exp": expires,
	}, ttl)
}

func (m *Manager) delConsentFromCache(ctx context.Context, userID uuid.UUID, bank auth.BankCode) {
	if m.redis == nil {
		return
	}
	_ = m.redis.Del(ctx, m.cacheKey(userID, bank))
}

// ---- misc ----

func isActive(status string, expires *time.Time) bool {
	if strings.EqualFold(status, "active") || strings.EqualFold(status, "approved") {
		if expires == nil {
			return true
		}
		return time.Now().Before(*expires)
	}
	return false
}

func normalizeStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "approved", "active":
		return "active"
	case "pending":
		return "pending"
	case "revoked":
		return "revoked"
	case "expired":
		return "expired"
	default:
		return s
	}
}

func parseTimePtr(s *string) *time.Time {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}
	return &t
}

func drainAndClose(rc io.ReadCloser) {
	if rc == nil {
		return
	}
	_, _ = io.Copy(io.Discard, rc)
	_ = rc.Close()
}
