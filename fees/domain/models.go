package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Currency string

const (
	CurrencyGEL Currency = "GEL"
	CurrencyUSD Currency = "USD"
)

type BillStatus string

const (
	StatusOpen   BillStatus = "OPEN"
	StatusClosed BillStatus = "CLOSED"
)

type Money struct {
	Currency Currency `json:"currency"`
	Amount   int64    `json:"amount"`
}

type LineItem struct {
	ID            string    `json:"id"`
	BillID        string    `json:"bill_id"`
	TransactionID string    `json:"transaction_id"`
	Description   string    `json:"description"`
	Currency      Currency  `json:"currency"`
	Amount        int64     `json:"amount"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
}

type Bill struct {
	ID          string     `json:"id"`
	OwnerID     string     `json:"owner_id"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`
	Status      BillStatus `json:"status"`
	Currency    Currency   `json:"currency"`
	Total       []Money    `json:"total"`
	LineItems   []LineItem `json:"line_items"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	Version     int64      `json:"version"`
	WorkflowID  string     `json:"workflow_id"`
}

type Invoice struct {
	ID        string     `json:"id"`
	Status    BillStatus `json:"status"`
	Currency  Currency   `json:"currency"`
	Total     []Money    `json:"total"`
	LineItems []LineItem `json:"line_items"`
	ClosedAt  time.Time  `json:"closed_at"`
	Version   int64      `json:"version"`
}

const (
	maxInt64 = int64(^uint64(0) >> 1)
	minInt64 = -maxInt64 - 1
)

func ValidateCreateBill(ownerID string, currency Currency, periodStart, periodEnd time.Time) error {
	if strings.TrimSpace(ownerID) == "" {
		return errors.New("owner_id is required")
	}
	if !IsSupportedCurrency(currency) {
		return fmt.Errorf("%w: %s", ErrUnsupportedCurrency, currency)
	}
	if periodEnd.IsZero() || periodStart.IsZero() || !periodEnd.After(periodStart) {
		return errors.New("period_end must be after period_start")
	}
	return nil
}

func ValidateAddLineItem(description string, currency Currency, amount int64) error {
	if strings.TrimSpace(description) == "" {
		return errors.New("description is required")
	}
	if !IsSupportedCurrency(currency) {
		return fmt.Errorf("%w: %s", ErrUnsupportedCurrency, currency)
	}
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	return nil
}

func ValidateIdempotencyKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("Idempotency-Key is required")
	}
	if len(key) > 255 {
		return errors.New("Idempotency-Key is too long")
	}
	return nil
}

func IsSupportedCurrency(currency Currency) bool {
	return currency == CurrencyGEL || currency == CurrencyUSD
}

func AggregateBillTotal(items []LineItem, currency Currency) ([]Money, error) {
	if !IsSupportedCurrency(currency) {
		return nil, ErrUnsupportedCurrency
	}
	total := int64(0)
	for _, item := range items {
		if item.Currency != currency {
			return nil, ErrCurrencyMismatch
		}
		if item.Amount > 0 && total > maxInt64-item.Amount {
			return nil, ErrAmountOverflow
		}
		if item.Amount < 0 && total < minInt64-item.Amount {
			return nil, ErrAmountOverflow
		}
		total += item.Amount
	}
	return []Money{{Currency: currency, Amount: total}}, nil
}

func CanonicalHash(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
