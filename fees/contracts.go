package fees

import (
	"time"

	"encore.app/fees/database"
	"encore.app/fees/domain"
	temporalfx "encore.app/fees/temporal"
	"go.temporal.io/sdk/workflow"
)

type Currency = domain.Currency
type BillStatus = domain.BillStatus
type Repository = database.Repository
type CreateBillRecord = database.CreateBillRecord
type AppendLineItemRecord = database.AppendLineItemRecord
type OutboxEvent = database.OutboxEvent
type WorkflowInput = temporalfx.WorkflowInput
type AddLineItemCommand = temporalfx.AddLineItemCommand
type Activities = temporalfx.Activities

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

type CreateBillRequest struct {
	OwnerID        string    `json:"owner_id"`
	Currency       Currency  `json:"currency"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	IdempotencyKey string    `header:"Idempotency-Key"`
}

type AddLineItemRequest struct {
	BillID         string   `path:"billID"`
	Description    string   `json:"description"`
	Currency       Currency `json:"currency"`
	Amount         int64    `json:"amount"`
	Source         string   `json:"source"`
	IdempotencyKey string   `header:"Idempotency-Key"`
}

type CloseBillRequest struct {
	BillID         string `path:"billID"`
	IdempotencyKey string `header:"Idempotency-Key"`
}

const (
	CurrencyGEL  = domain.CurrencyGEL
	CurrencyUSD  = domain.CurrencyUSD
	StatusOpen   = domain.StatusOpen
	StatusClosed = domain.StatusClosed
)

var (
	ErrNotFound            = domain.ErrNotFound
	ErrBillClosed          = domain.ErrBillClosed
	ErrConflict            = domain.ErrConflict
	ErrInvalidArgument     = domain.ErrInvalidArgument
	ErrUnsupportedCurrency = domain.ErrUnsupportedCurrency
	ErrAmountOverflow      = domain.ErrAmountOverflow
	ErrCurrencyMismatch    = domain.ErrCurrencyMismatch
)

func ValidateCreateBill(req CreateBillRequest) error {
	return domain.ValidateCreateBill(req.OwnerID, req.Currency, req.PeriodStart, req.PeriodEnd)
}

func ValidateAddLineItem(req AddLineItemRequest) error {
	return domain.ValidateAddLineItem(req.Description, req.Currency, req.Amount)
}

func ValidateIdempotencyKey(key string) error {
	return domain.ValidateIdempotencyKey(key)
}

func CanonicalHash(value any) (string, error) {
	return domain.CanonicalHash(value)
}

func workflowIDForBill(id string) string {
	return temporalfx.WorkflowIDForBill(id)
}

func BillWorkflow(ctx workflow.Context, input WorkflowInput) (Invoice, error) {
	invoice, err := temporalfx.BillWorkflow(ctx, input)
	return invoiceFromDomain(invoice), err
}

func billFromDomain(value domain.Bill) Bill {
	return Bill{
		ID: value.ID, OwnerID: value.OwnerID, PeriodStart: value.PeriodStart, PeriodEnd: value.PeriodEnd,
		Status: value.Status, Currency: value.Currency, Total: moneyFromDomain(value.Total),
		LineItems: lineItemsFromDomain(value.LineItems), ClosedAt: value.ClosedAt, Version: value.Version, WorkflowID: value.WorkflowID,
	}
}

func invoiceFromDomain(value domain.Invoice) Invoice {
	return Invoice{
		ID: value.ID, Status: value.Status, Currency: value.Currency, Total: moneyFromDomain(value.Total),
		LineItems: lineItemsFromDomain(value.LineItems), ClosedAt: value.ClosedAt, Version: value.Version,
	}
}

func lineItemFromDomain(value domain.LineItem) LineItem {
	return LineItem{
		ID: value.ID, BillID: value.BillID, TransactionID: value.TransactionID, Description: value.Description,
		Currency: value.Currency, Amount: value.Amount, Source: value.Source, CreatedAt: value.CreatedAt,
	}
}

func domainLineItem(value LineItem) domain.LineItem {
	return domain.LineItem{
		ID: value.ID, BillID: value.BillID, TransactionID: value.TransactionID, Description: value.Description,
		Currency: value.Currency, Amount: value.Amount, Source: value.Source, CreatedAt: value.CreatedAt,
	}
}

func moneyFromDomain(values []domain.Money) []Money {
	result := make([]Money, len(values))
	for i, value := range values {
		result[i] = Money{Currency: value.Currency, Amount: value.Amount}
	}
	return result
}

func lineItemsFromDomain(values []domain.LineItem) []LineItem {
	result := make([]LineItem, len(values))
	for i, value := range values {
		result[i] = lineItemFromDomain(value)
	}
	return result
}
