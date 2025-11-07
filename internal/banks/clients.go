package banks

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/auth"
	"github.com/google/uuid"
)

type Money struct {
    AmountMinor int64       `json:"amount_minor"`// 100 = 1.00 | always >= 0
    Currency    string      `json:"currency"`// 'RUB', 'EUR', ...
}

func ParseMoney(s, cur string) (Money, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return Money{}, err
	}
	return MoneyFromFloat(f, cur)
}

func MoneyFromFloat(f float64, cur string) (Money, error) {
	if f < 0 {
		return Money{}, fmt.Errorf("invalid amount: less than 0")
	}
	minor := int64(math.Round(f * 100))
	if minor < 0 {
		return Money{}, fmt.Errorf("invalid amount: negative minor units")
	}
	return Money{
		AmountMinor: minor,
		Currency:    strings.ToUpper(cur),
	}, nil
}

func MoneyFromMinor(minor int64, cur string) (Money, error) {
	if minor < 0 {
		return Money{}, fmt.Errorf("invalid amount: negative minor units")
	}
	return Money{
		AmountMinor: minor,
		Currency:    strings.ToUpper(cur),
	}, nil
}

type Direction string

const (
	Debit 		Direction = "debit"
	Credit 		Direction = "credit"
)

type Account struct {
	ID        string        `json:"id"`
	Name      string        `json:"name,omitempty"`
	IBAN      string        `json:"iban,omitempty"`
	Currency  string        `json:"currency"` // валюта счёта
	Balance   Money         `json:"balance"`
	Bank      auth.BankCode `json:"bank"`
	UpdatedAt time.Time     `json:"updated_at,omitempty"`
}


type Transaction struct {
    ID          string        `json:"id"`
    AccountID   string        `json:"account_id"`
    Amount      Money         `json:"amount"`
    Direction   Direction     `json:"direction"`
    Merchant    string        `json:"merchant,omitempty"`
    Description string        `json:"description,omitempty"`
    Bank        auth.BankCode `json:"bank"`
    BookedAt    time.Time     `json:"booked_at"`
}


type Client interface {
	Accounts(ctx context.Context, userID uuid.UUID) ([]Account, error)
	Transactions(ctx context.Context, userID uuid.UUID, since time.Time, limit int) ([]Transaction, error)
}

