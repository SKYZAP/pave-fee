package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"encore.app/fees/domain"
	"encore.dev/et"
	"github.com/google/uuid"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := et.NewTestDatabase(context.Background(), "fees")
	if err != nil {
		t.Fatal(err)
	}
	return &Repository{DB: db}
}

func createTestBill(t *testing.T, repo *Repository) domain.Bill {
	t.Helper()
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	bill, err := repo.CreateBill(context.Background(), CreateBillRecord{
		ID: uuid.New(), OwnerID: "integration-owner", Currency: domain.CurrencyUSD,
		PeriodStart: start, PeriodEnd: start.Add(31 * 24 * time.Hour),
		IdempotencyKey: uuid.NewString(), RequestHash: "create-hash", WorkflowID: "bill-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return bill
}

func TestRepositoryClosePersistsSingleCurrencyInvoiceAndOutbox(t *testing.T) {
	repo := newTestRepository(t)
	bill := createTestBill(t, repo)
	ctx := context.Background()
	billID := uuid.MustParse(bill.ID)

	for _, item := range []AppendLineItemRecord{
		{BillID: billID, TransactionID: "usd-1", RequestHash: "usd-hash", Description: "USD usage", Currency: domain.CurrencyUSD, Amount: 1750, Source: "test"},
		{BillID: billID, TransactionID: "usd-2", RequestHash: "usd-2-hash", Description: "USD usage", Currency: domain.CurrencyUSD, Amount: 2000, Source: "test"},
	} {
		if _, err := repo.AppendLineItem(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	invoice, err := repo.CloseBill(ctx, billID)
	if err != nil {
		t.Fatal(err)
	}
	if len(invoice.Total) != 1 || invoice.Total[0] != (domain.Money{Currency: domain.CurrencyUSD, Amount: 3750}) {
		t.Fatalf("unexpected totals: %#v", invoice.Total)
	}
	events, err := repo.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != "bill.closed" {
		t.Fatalf("unexpected outbox events: %#v", events)
	}
}

func TestRepositoryRejectsClosedAndMismatchedCurrencies(t *testing.T) {
	repo := newTestRepository(t)
	bill := createTestBill(t, repo)
	billID := uuid.MustParse(bill.ID)
	if _, err := repo.AppendLineItem(context.Background(), AppendLineItemRecord{
		BillID: billID, TransactionID: "gel-item", RequestHash: "gel-hash",
		Description: "GEL usage", Currency: domain.CurrencyGEL, Amount: 100, Source: "test",
	}); !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("got %v, want %v", err, domain.ErrCurrencyMismatch)
	}
	if _, err := repo.CloseBill(context.Background(), billID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AppendLineItem(context.Background(), AppendLineItemRecord{
		BillID: billID, TransactionID: "late-item", RequestHash: "late-hash",
		Description: "late", Currency: domain.CurrencyUSD, Amount: 1, Source: "test",
	}); !errors.Is(err, domain.ErrBillClosed) {
		t.Fatalf("got %v, want %v", err, domain.ErrBillClosed)
	}
}
