package vbank

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/auth"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/banks"
	"github.com/google/uuid"
)

type Client struct {
	req *banks.Requester
}

func New(req *banks.Requester) *Client { return &Client{req: req} }

// Accounts запрашивает список счетов и для каждого подтягивает баланс.
// Требует согласия (NeedConsent=true).
func (c *Client) Accounts(ctx context.Context, userID uuid.UUID) ([]banks.Account, error) {
	resp, err := c.req.Do(ctx, banks.Params{
		Method:      http.MethodGet,
		Path:        "/accounts",
		Query:       map[string]string{"client_id": userID.String()},
		NeedConsent: true,
		UserID:      userID,
	})
	if err != nil {
		return nil, err
	}
	defer closeBody(resp.Body)

	if resp.StatusCode/100 != 2 {
		return nil, httpError("vbank accounts", resp)
	}

	var ar AccountsResp
	if err := decodeJSON(resp.Body, &ar); err != nil {
		return nil, fmt.Errorf("vbank accounts decode: %w", err)
	}

	items := ar.Flat()
	out := make([]banks.Account, 0, len(items))
	now := time.Now()

	for _, a := range items {
		acc := banks.Account{
			ID:        or(a.AccountID, a.ID),
			Name:      a.Name,
			IBAN:      a.IBAN,
			Currency:  a.Currency,
			Bank:      auth.BankV,
			UpdatedAt: now,
		}
		if acc.ID != "" {
			if bal, ok, err := c.fetchBalance(ctx, userID, acc.ID); err == nil && ok {
				acc.Balance = bal
			}
		}
		out = append(out, acc)
	}
	return out, nil
}

// Transactions запрашивает операции. Требует согласия.
func (c *Client) Transactions(ctx context.Context, userID uuid.UUID, since time.Time, limit int) ([]banks.Transaction, error) {
	q := map[string]string{
		"client_id": userID.String(),
		"since":     since.UTC().Format(time.RFC3339),
	}
	if limit > 0 {
		q["limit"] = strconv.Itoa(limit)
	}

	resp, err := c.req.Do(ctx, banks.Params{
		Method:      http.MethodGet,
		Path:        "/transactions",
		Query:       q,
		NeedConsent: true,
		UserID:      userID,
	})
	if err != nil {
		return nil, err
	}
	defer closeBody(resp.Body)

	if resp.StatusCode/100 != 2 {
		return nil, httpError("vbank transactions", resp)
	}

	var tr TxResp
	if err := decodeJSON(resp.Body, &tr); err != nil {
		return nil, fmt.Errorf("vbank tx decode: %w", err)
	}

	out := make([]banks.Transaction, 0, len(tr.Flat()))
	for _, t := range tr.Flat() {
		amt, sign := moneyFromAny(t.Amount, t.Currency) // sign: -1,0,+1
		dir := mapDir(t.Direction, sign)
		out = append(out, banks.Transaction{
			ID:          or(t.TransactionID, t.ID),
			AccountID:   or(t.AccountID, t.AccID),
			Amount:      amt,
			Direction:   dir,
			Merchant:    t.Merchant,
			Description: t.Description,
			Bank:        auth.BankV,
			BookedAt:    t.BookingDateTime,
		})
	}
	return out, nil
}

// --- private helpers ---

func (c *Client) fetchBalance(ctx context.Context, userID uuid.UUID, accountID string) (banks.Money, bool, error) {
	resp, err := c.req.Do(ctx, banks.Params{
		Method:      http.MethodGet,
		Path:        "/accounts/%s/balances",
		PathArgs:    []any{accountID},
		NeedConsent: true,
		UserID:      userID,
	})
	if err != nil {
		return banks.Money{}, false, err
	}
	defer closeBody(resp.Body)

	if resp.StatusCode/100 != 2 {
		return banks.Money{}, false, nil
	}

	var br BalancesResp
	if err := decodeJSON(resp.Body, &br); err != nil {
		return banks.Money{}, false, err
	}

	if b, ok := br.AvailableOrCurrent(); ok {
		m, _ := banks.MoneyFromFloat(abs(toFloat(b.Amount)), up(b.Currency))
		return m, true, nil
	}
	return banks.Money{}, false, nil
}

// --- DTO & util ---

type AccountsResp struct {
	Accounts []AccountDTO `json:"accounts,omitempty"`
	Data     *struct {
		Accounts []AccountDTO `json:"accounts"`
	} `json:"data,omitempty"`
}
func (r AccountsResp) Flat() []AccountDTO {
	if len(r.Accounts) > 0 {
		return r.Accounts
	}
	if r.Data != nil && len(r.Data.Accounts) > 0 {
		return r.Data.Accounts
	}
	return nil
}

type AccountDTO struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"`
	Name      string `json:"name"`
	IBAN      string `json:"iban"`
	Currency  string `json:"currency"`
}

type BalancesResp struct {
	Balances []BalanceItem `json:"balances,omitempty"`
	Data     *struct {
		Balances []BalanceItem `json:"balances"`
	} `json:"data,omitempty"`
}
func (b BalancesResp) AvailableOrCurrent() (BalanceItem, bool) {
	list := b.Balances
	if len(list) == 0 && b.Data != nil {
		list = b.Data.Balances
	}
	var cur *BalanceItem
	for i := range list {
		switch strings.ToLower(list[i].Type) {
		case "available":
			return list[i], true
		case "current":
			cur = &list[i]
		}
	}
	if cur != nil {
		return *cur, true
	}
	return BalanceItem{}, false
}
type BalanceItem struct {
	Type     string `json:"type"`               // "available" | "current"
	Amount   any    `json:"amount"`             // "100.25" | 100.25 | 100
	Currency string `json:"currency"`           // "RUB"
}

type TxResp struct {
	Transactions []TxItem `json:"transactions,omitempty"`
	Data         *struct {
		Transactions []TxItem `json:"transactions"`
	} `json:"data,omitempty"`
}
func (t TxResp) Flat() []TxItem {
	if len(t.Transactions) > 0 {
		return t.Transactions
	}
	if t.Data != nil && len(t.Data.Transactions) > 0 {
		return t.Data.Transactions
	}
	return nil
}

type TxItem struct {
	ID              string    `json:"id"`
	TransactionID   string    `json:"transactionId"`
	AccountID       string    `json:"accountId"`
	AccID           string    `json:"accId"`
	Amount          any       `json:"amount"`
	Currency        string    `json:"currency"`
	Direction       string    `json:"direction"`        // "credit" | "debit" (может отсутствовать)
	BookingDateTime time.Time `json:"bookingDateTime"`  // ISO8601
	Merchant        string    `json:"merchant"`
	Description     string    `json:"description"`
}

// shared utils (локально, чтобы не плодить зависимости)

func decodeJSON(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }
func closeBody(rc io.ReadCloser)           { if rc != nil { _, _ = io.Copy(io.Discard, rc); _ = rc.Close() } }
func httpError(where string, resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("%s: no response", where)
	}
	return fmt.Errorf("%s: status=%d %s", where, resp.StatusCode, resp.Status)
}

func or(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func up(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

func toFloat(v any) float64 {
	switch t := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	case float64:
		return t
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}
func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func moneyFromAny(v any, cur string) (banks.Money, int) {
	f := toFloat(v)
	sign := 0
	switch {
	case f > 0:
		sign = +1
	case f < 0:
		sign = -1
	}
	m, _ := banks.MoneyFromFloat(abs(f), up(cur))
	return m, sign
}

func mapDir(s string, sign int) banks.Direction {
	ls := strings.ToLower(strings.TrimSpace(s))
	switch ls {
	case "credit":
		return banks.Credit
	case "debit":
		return banks.Debit
	}
	// если direction не пришёл — определяем по знаку суммы
	if sign >= 0 {
		return banks.Credit
	}
	return banks.Debit
}
