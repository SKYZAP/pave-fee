package domain

import (
	"errors"
	"testing"
	"time"
)

func TestValidateCreateBillRejectsInvalidPeriod(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if err := ValidateCreateBill("merchant_123", CurrencyUSD, start, start); err == nil {
		t.Fatal("expected invalid period error")
	}
}

func TestAggregateBillTotalUsesBillCurrency(t *testing.T) {
	got, err := AggregateBillTotal([]LineItem{
		{Currency: CurrencyUSD, Amount: 1250},
		{Currency: CurrencyUSD, Amount: 750},
	}, CurrencyUSD)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (Money{Currency: CurrencyUSD, Amount: 2000}) {
		t.Fatalf("unexpected total: %#v", got)
	}
}

func TestAggregateBillTotalRejectsInvalidAmountsAndCurrencies(t *testing.T) {
	_, err := AggregateBillTotal([]LineItem{
		{Currency: CurrencyUSD, Amount: 1<<63 - 1},
		{Currency: CurrencyUSD, Amount: 1},
	}, CurrencyUSD)
	if !errors.Is(err, ErrAmountOverflow) {
		t.Fatalf("got %v, want %v", err, ErrAmountOverflow)
	}

	_, err = AggregateBillTotal([]LineItem{{Currency: CurrencyGEL, Amount: 100}}, CurrencyUSD)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("got %v, want %v", err, ErrCurrencyMismatch)
	}
}

func TestValidateLineItemRejectsUnsupportedCurrency(t *testing.T) {
	err := ValidateAddLineItem("usage", Currency("EUR"), 100)
	if !errors.Is(err, ErrUnsupportedCurrency) {
		t.Fatalf("got %v, want %v", err, ErrUnsupportedCurrency)
	}
}
