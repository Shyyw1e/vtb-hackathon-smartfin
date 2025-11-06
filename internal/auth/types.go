package auth

import "context"

type BankCode string

const (
	BankV BankCode = "vbank"
	BankA BankCode = "abank"
	BankS BankCode = "sbank"
)

type TokenProvider interface {
	GetToken(ctx context.Context, bank BankCode) (string, error)     // быстрый: достаёт из кеша, при необходимости обновляет
	ForceRefresh(ctx context.Context, bank BankCode) (string, error) // жёсткий рефреш
}