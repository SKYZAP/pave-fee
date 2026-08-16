package domain

import "errors"

var (
	ErrNotFound            = errors.New("bill not found")
	ErrBillClosed          = errors.New("bill_closed")
	ErrConflict            = errors.New("idempotency conflict")
	ErrInvalidArgument     = errors.New("invalid argument")
	ErrUnsupportedCurrency = errors.New("unsupported_currency")
	ErrAmountOverflow      = errors.New("amount overflow")
	ErrCurrencyMismatch    = errors.New("currency_mismatch")
)
