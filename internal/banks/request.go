package banks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/auth"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/config"
	"github.com/google/uuid"
)

var (
	ErrConsentPending = errors.New("consent pending, user interaction required")
	ErrUnauthorized = errors.New("unauthorized after token refresh")
	ErrRateLimited = errors.New("rate limited after retries")
	ErrConsenterNotConfigured = errors.New("consenter is not configured but consent is required")
)

type Consenter interface {
	EnsureConsent(ctx context.Context, userID uuid.UUID, bank auth.BankCode) (string, error)
}

type Requester struct {
	Bank        auth.BankCode
	BaseURL     string
	CFG         *config.Config
	HTTPC       *http.Client
	TP          auth.TokenProvider
	TeamID      string
	Timeout     time.Duration
	MaxAttempts int
	BaseBackoff time.Duration
	Consenter   Consenter
}

type RequesterOption func(*Requester)

func WithConsenter(c Consenter) RequesterOption {
	return func(r *Requester) { r.Consenter = c }
}
func WithAttempts(n int) RequesterOption {
	return func(r *Requester) {
		if n > 0 {
			r.MaxAttempts = n
		}
	}
}
func WithBaseBackoff(d time.Duration) RequesterOption {
	return func(r *Requester) {
		if d > 0 {
			r.BaseBackoff = d
		}
	}
}
func WithTimeout(d time.Duration) RequesterOption {
	return func(r *Requester) {
		if d > 0 {
			r.Timeout = d
		}
	}
}

func NewRequester(
	bank auth.BankCode,
	baseURL string,
	httpc *http.Client,
	tp auth.TokenProvider,
	cfg *config.Config,
	opts ...RequesterOption,
) (*Requester, error) {
	if httpc == nil {
		return nil, fmt.Errorf("http client is nil")
	}
	if tp == nil {
		return nil, fmt.Errorf("token provider is nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	trim := strings.TrimRight(baseURL, "/")
	if trim == "" {
		return nil, fmt.Errorf("baseURL is empty")
	}
	base := trim + "/"

	r := &Requester{
		Bank:        bank,
		BaseURL:     base,
		CFG:         cfg,
		HTTPC:       httpc,
		TP:          tp,
		TeamID:      cfg.App.TeamID, 
		Timeout:     time.Duration(cfg.Limits.BankTimeoutSec) * time.Second,
		MaxAttempts: 3,
		BaseBackoff: 200 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.Timeout <= 0 {
		r.Timeout = 4 * time.Second
	}
	if r.MaxAttempts <= 0 {
		r.MaxAttempts = 3
	}
	if r.BaseBackoff <= 0 {
		r.BaseBackoff = 200 * time.Millisecond
	}
	return r, nil
}

type Params struct {
	Method          string
	Path            string            
	PathArgs        []any             
	Query           map[string]string 
	Headers         map[string]string 
	Body            []byte            
	ContentType     string            
	TimeoutOverride time.Duration     

	NeedConsent bool
	UserID      uuid.UUID

	IdempotencyKey string
}

func (r *Requester) Do(ctx context.Context, p Params) (*http.Response, error) {
	if p.Method == "" {
		return nil, fmt.Errorf("method is required")
	}
	fullURL := buildURL(r.BaseURL, p.Path, p.PathArgs, p.Query)

	timeout := r.Timeout
	if p.TimeoutOverride > 0 {
		timeout = p.TimeoutOverride
	}
	ctxReq, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	attempts := r.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	var (
		didRefresh       bool
		didEnsureConsent bool
		lastResp         *http.Response
	)

	for attempt := 0; attempt < attempts; attempt++ {
		var bodyRC io.ReadCloser
		if len(p.Body) > 0 {
			bodyRC = io.NopCloser(bytes.NewReader(p.Body))
		}

		req, err := http.NewRequestWithContext(ctxReq, p.Method, fullURL, bodyRC)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Accept", "application/json")
		if p.ContentType != "" && len(p.Body) > 0 {
			req.Header.Set("Content-Type", p.ContentType)
		}
		if p.IdempotencyKey != "" {
			req.Header.Set("Idempotency-Key", p.IdempotencyKey)
		}
		if rid := requestIDFromCtx(ctx); rid != "" {
			req.Header.Set("X-Request-ID", rid)
		}
		for k, v := range p.Headers {
			if v == "" {
				continue
			}
			req.Header.Set(k, v)
		}

		token, err := r.TP.GetToken(ctxReq, r.Bank)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		if p.NeedConsent {
			if r.Consenter == nil {
				return nil, ErrConsenterNotConfigured
			}
			if r.TeamID == "" {
				return nil, fmt.Errorf("team id is empty: set APP_TEAM_ID or app.team_id")
			}
			cid, err := r.Consenter.EnsureConsent(ctxReq, p.UserID, r.Bank)
			if err != nil {
				if errors.Is(err, ErrConsentPending) {
					return nil, ErrConsentPending
				}
				return nil, err
			}
			req.Header.Set("X-Requesting-Bank", r.TeamID)
			req.Header.Set("X-Consent-Id", cid)
		}

		resp, err := r.HTTPC.Do(req)
		if err != nil {
			if isRetryableNetErr(err) && attempt < attempts-1 {
				sleepWithJitter(r.BaseBackoff, attempt)
				continue
			}
			return nil, err
		}

		lastResp = resp

		status := resp.StatusCode
		switch {
		case status >= 200 && status < 300:
			return resp, nil

		case status == http.StatusUnauthorized: // 401
			if !didRefresh {
				closeQuiet(resp)
				if _, err := r.TP.ForceRefresh(ctxReq, r.Bank); err != nil {
					return nil, ErrUnauthorized
				}
				didRefresh = true
				sleepWithJitter(r.BaseBackoff, attempt)
				continue
			}
			return resp, nil

		case status == http.StatusForbidden: // 403
			if p.NeedConsent && !didEnsureConsent {
				closeQuiet(resp)
				if r.Consenter == nil {
					return nil, ErrConsenterNotConfigured
				}
				if _, err := r.Consenter.EnsureConsent(ctxReq, p.UserID, r.Bank); err != nil {
					if errors.Is(err, ErrConsentPending) {
						return nil, ErrConsentPending
					}
					return nil, err
				}
				didEnsureConsent = true
				sleepWithJitter(r.BaseBackoff, attempt)
				continue
			}
			return resp, nil

		case status == http.StatusTooManyRequests: // 429
			retryAfter := parseRetryAfter(resp)
			closeQuiet(resp)
			if attempt < attempts-1 {
				if retryAfter <= 0 {
					retryAfter = 100 * time.Millisecond
				}
				if retryAfter > 5*time.Second {
					retryAfter = 5 * time.Second
				}
				time.Sleep(retryAfter)
				continue
			}
			return nil, ErrRateLimited

		case status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout: // 502/503/504
			closeQuiet(resp)
			if attempt < attempts-1 {
				sleepWithJitter(r.BaseBackoff, attempt)
				continue
			}
			return lastResp, nil

		default:
			return resp, nil
		}
	}

	if lastResp != nil {
		return lastResp, nil
	}
	return nil, fmt.Errorf("request failed without response")
}

func buildURL(base, path string, args []any, q map[string]string) string {
	p := path
	if len(args) > 0 {
		p = fmt.Sprintf(path, args...)
	}
	u := strings.TrimRight(base, "/") + "/" + strings.TrimLeft(p, "/")
	if len(q) == 0 {
		return u
	}
	values := url.Values{}
	for k, v := range q {
		if v == "" {
			continue
		}
		values.Set(k, v)
	}
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + values.Encode()
}

type ctxKey string

const requestIDKey ctxKey = "x-request-id"

func requestIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(requestIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func isRetryableNetErr(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		if ne.Timeout() || ne.Temporary() {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "use of closed network connection"),
		strings.Contains(msg, "tls handshake timeout"),
		strings.Contains(msg, "no such host"),
		strings.Contains(msg, "server misbehaving"):
		return true
	default:
		return false
	}
}

func parseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	h := resp.Header.Get("Retry-After")
	if h == "" {
		return 0
	}
	if secs, err := time.ParseDuration(h + "s"); err == nil {
		return secs
	}
	if when, err := http.ParseTime(h); err == nil {
		d := time.Until(when)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func sleepWithJitter(base time.Duration, attempt int) {
	if base <= 0 {
		base = 200 * time.Millisecond
	}
	backoff := base << attempt
	jitter := time.Duration(rand.Int63n(int64(backoff/3))) - backoff/6
	time.Sleep(backoff + jitter)
}

func closeQuiet(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}