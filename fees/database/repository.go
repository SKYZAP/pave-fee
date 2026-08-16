package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"encore.app/fees/domain"
	"encore.dev/storage/sqldb"
	"github.com/google/uuid"
)

type Repository struct {
	DB *sqldb.Database
}

type CreateBillRecord struct {
	ID             uuid.UUID
	OwnerID        string
	Currency       domain.Currency
	PeriodStart    time.Time
	PeriodEnd      time.Time
	IdempotencyKey string
	RequestHash    string
	WorkflowID     string
}

type AppendLineItemRecord struct {
	BillID        uuid.UUID
	TransactionID string
	RequestHash   string
	Description   string
	Currency      domain.Currency
	Amount        int64
	Source        string
}

type OutboxEvent struct {
	EventID          uuid.UUID
	AggregateID      uuid.UUID
	AggregateVersion int64
	EventType        string
	Payload          []byte
}

func (r *Repository) ListBills(ctx context.Context, ownerID string) ([]domain.Bill, error) {
	rows, err := r.DB.Query(ctx, `SELECT id FROM bills WHERE owner_id = $1 ORDER BY period_start DESC, id`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bills := make([]domain.Bill, 0)
	for rows.Next() {
		var billID uuid.UUID
		if err := rows.Scan(&billID); err != nil {
			return nil, err
		}
		bill, err := r.GetBill(ctx, billID)
		if err != nil {
			return nil, err
		}
		bills = append(bills, bill)
	}
	return bills, rows.Err()
}

func (r *Repository) CreateBill(ctx context.Context, record CreateBillRecord) (domain.Bill, error) {
	existing, existingHash, lookupErr := r.findBillByKey(ctx, record.OwnerID, record.IdempotencyKey)
	if lookupErr == nil {
		if existingHash == record.RequestHash {
			return existing, nil
		}
		return domain.Bill{}, domain.ErrConflict
	}
	if !errors.Is(lookupErr, sqldb.ErrNoRows) {
		return domain.Bill{}, lookupErr
	}
	if !domain.IsSupportedCurrency(record.Currency) {
		return domain.Bill{}, domain.ErrUnsupportedCurrency
	}
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	_, err := r.DB.Exec(ctx, `
		INSERT INTO bills (id, owner_id, period_start, period_end, workflow_id, idempotent_key, request_hash, currency)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		record.ID, record.OwnerID, record.PeriodStart, record.PeriodEnd, record.WorkflowID,
		record.IdempotencyKey, record.RequestHash, record.Currency,
	)
	if err != nil {
		existing, existingHash, lookupErr = r.findBillByKey(ctx, record.OwnerID, record.IdempotencyKey)
		if lookupErr == nil {
			if existingHash == record.RequestHash {
				return existing, nil
			}
			return domain.Bill{}, domain.ErrConflict
		}
		return domain.Bill{}, err
	}
	return r.GetBill(ctx, record.ID)
}

func (r *Repository) GetBill(ctx context.Context, billID uuid.UUID) (domain.Bill, error) {
	var bill domain.Bill
	var storedID uuid.UUID
	var total []byte
	var closedAt *time.Time
	err := r.DB.QueryRow(ctx, `
		SELECT id, owner_id, period_start, period_end, status, closed_at, total, version, workflow_id, currency
		FROM bills WHERE id = $1`, billID,
	).Scan(&storedID, &bill.OwnerID, &bill.PeriodStart, &bill.PeriodEnd, &bill.Status,
		&closedAt, &total, &bill.Version, &bill.WorkflowID, &bill.Currency)
	if errors.Is(err, sqldb.ErrNoRows) {
		return domain.Bill{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, err
	}
	bill.ID = storedID.String()
	bill.ClosedAt = closedAt
	if len(total) > 0 {
		if err := json.Unmarshal(total, &bill.Total); err != nil {
			return domain.Bill{}, fmt.Errorf("decode bill total: %w", err)
		}
	}
	bill.LineItems, err = r.listLineItems(ctx, storedID)
	if err != nil {
		return domain.Bill{}, err
	}
	return bill, nil
}

func (r *Repository) AppendLineItem(ctx context.Context, record AppendLineItemRecord) (domain.LineItem, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return domain.LineItem{}, err
	}
	defer tx.Rollback()

	var status domain.BillStatus
	var billCurrency domain.Currency
	err = tx.QueryRow(ctx, `SELECT status, currency FROM bills WHERE id = $1 FOR UPDATE`, record.BillID).Scan(&status, &billCurrency)
	if errors.Is(err, sqldb.ErrNoRows) {
		return domain.LineItem{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.LineItem{}, err
	}
	if status == domain.StatusClosed {
		return domain.LineItem{}, domain.ErrBillClosed
	}
	if record.Currency != billCurrency {
		return domain.LineItem{}, domain.ErrCurrencyMismatch
	}

	var item domain.LineItem
	var storedHash string
	err = tx.QueryRow(ctx, `
		SELECT id, bill_id, transaction_id, request_hash, description, currency, amount, source, created_at
		FROM line_items WHERE bill_id = $1 AND transaction_id = $2`, record.BillID, record.TransactionID,
	).Scan(&item.ID, &item.BillID, &item.TransactionID, &storedHash, &item.Description,
		&item.Currency, &item.Amount, &item.Source, &item.CreatedAt)
	if err == nil {
		if storedHash != record.RequestHash {
			return domain.LineItem{}, domain.ErrConflict
		}
		return item, nil
	}
	if !errors.Is(err, sqldb.ErrNoRows) {
		return domain.LineItem{}, err
	}

	itemID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO line_items (id, bill_id, transaction_id, request_hash, description, currency, amount, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		itemID, record.BillID, record.TransactionID, record.RequestHash, record.Description,
		record.Currency, record.Amount, record.Source,
	)
	if err != nil {
		return domain.LineItem{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE bills SET version = version + 1, updated_at = NOW() WHERE id = $1`, record.BillID); err != nil {
		return domain.LineItem{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.LineItem{}, err
	}
	return domain.LineItem{
		ID: itemID.String(), BillID: record.BillID.String(), TransactionID: record.TransactionID,
		Description: record.Description, Currency: record.Currency, Amount: record.Amount,
		Source: record.Source, CreatedAt: time.Now().UTC(),
	}, nil
}

func (r *Repository) CloseBill(ctx context.Context, billID uuid.UUID) (domain.Invoice, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return domain.Invoice{}, err
	}
	defer tx.Rollback()

	var status domain.BillStatus
	var storedTotal []byte
	var closedAt *time.Time
	var version int64
	var currency domain.Currency
	err = tx.QueryRow(ctx, `
		SELECT status, total, closed_at, version, currency FROM bills WHERE id = $1 FOR UPDATE`, billID,
	).Scan(&status, &storedTotal, &closedAt, &version, &currency)
	if errors.Is(err, sqldb.ErrNoRows) {
		return domain.Invoice{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Invoice{}, err
	}
	if status == domain.StatusClosed {
		items, err := r.listLineItemsTx(ctx, tx, billID)
		return decodeInvoice(billID, storedTotal, closedAt, version, currency, items, err)
	}
	items, err := r.listLineItemsTx(ctx, tx, billID)
	if err != nil {
		return domain.Invoice{}, err
	}
	totals, err := domain.AggregateBillTotal(items, currency)
	if err != nil {
		return domain.Invoice{}, err
	}
	totalJSON, err := json.Marshal(totals)
	if err != nil {
		return domain.Invoice{}, err
	}
	now := time.Now().UTC()
	version++
	_, err = tx.Exec(ctx, `
		UPDATE bills SET status = 'CLOSED', total = $2, closed_at = $3, version = $4, updated_at = $3 WHERE id = $1`,
		billID, totalJSON, now, version,
	)
	if err != nil {
		return domain.Invoice{}, err
	}
	eventID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("bill.closed:"+billID.String()+":"+fmt.Sprint(version)))
	invoice := domain.Invoice{ID: billID.String(), Status: domain.StatusClosed, Currency: currency, Total: totals, LineItems: items, ClosedAt: now, Version: version}
	payload, err := json.Marshal(invoice)
	if err != nil {
		return domain.Invoice{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (event_id, aggregate_type, aggregate_id, event_type, event_version, aggregate_version, payload)
		VALUES ($1, 'bill', $2, 'bill.closed', 'v1', $3, $4) ON CONFLICT DO NOTHING`,
		eventID, billID, version, payload,
	)
	if err != nil {
		return domain.Invoice{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Invoice{}, err
	}
	return invoice, nil
}

func (r *Repository) ListPendingOutbox(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT event_id, aggregate_id, aggregate_version, event_type, payload
		FROM outbox_events WHERE status = 'PENDING' AND available_at <= NOW()
		ORDER BY created_at, event_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []OutboxEvent
	for rows.Next() {
		var event OutboxEvent
		if err := rows.Scan(&event.EventID, &event.AggregateID, &event.AggregateVersion, &event.EventType, &event.Payload); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *Repository) MarkOutboxPublished(ctx context.Context, eventID uuid.UUID, publishedAt time.Time) error {
	_, err := r.DB.Exec(ctx, `UPDATE outbox_events SET status = 'PUBLISHED', published_at = $2 WHERE event_id = $1`, eventID, publishedAt)
	return err
}

func (r *Repository) findBillByKey(ctx context.Context, ownerID, key string) (domain.Bill, string, error) {
	var id uuid.UUID
	var requestHash string
	err := r.DB.QueryRow(ctx, `SELECT id, request_hash FROM bills WHERE owner_id = $1 AND idempotent_key = $2`, ownerID, key).Scan(&id, &requestHash)
	if err != nil {
		return domain.Bill{}, "", err
	}
	bill, err := r.GetBill(ctx, id)
	return bill, requestHash, err
}

func (r *Repository) listLineItems(ctx context.Context, billID uuid.UUID) ([]domain.LineItem, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT id, bill_id, transaction_id, description, currency, amount, source, created_at
		FROM line_items WHERE bill_id = $1 ORDER BY created_at, id`, billID)
	if err != nil {
		return nil, err
	}
	return scanLineItems(rows)
}

func (r *Repository) listLineItemsTx(ctx context.Context, tx *sqldb.Tx, billID uuid.UUID) ([]domain.LineItem, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, bill_id, transaction_id, description, currency, amount, source, created_at
		FROM line_items WHERE bill_id = $1 ORDER BY created_at, id`, billID)
	if err != nil {
		return nil, err
	}
	return scanLineItems(rows)
}

func scanLineItems(rows *sqldb.Rows) ([]domain.LineItem, error) {
	defer rows.Close()
	items := make([]domain.LineItem, 0)
	for rows.Next() {
		var item domain.LineItem
		if err := rows.Scan(&item.ID, &item.BillID, &item.TransactionID, &item.Description, &item.Currency, &item.Amount, &item.Source, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func decodeInvoice(billID uuid.UUID, totalJSON []byte, closedAt *time.Time, version int64, currency domain.Currency, items []domain.LineItem, err error) (domain.Invoice, error) {
	if err != nil {
		return domain.Invoice{}, err
	}
	var totals []domain.Money
	if err := json.Unmarshal(totalJSON, &totals); err != nil {
		return domain.Invoice{}, err
	}
	return domain.Invoice{
		ID: billID.String(), Status: domain.StatusClosed, Currency: currency,
		Total: totals, LineItems: items, ClosedAt: *closedAt, Version: version,
	}, nil
}
