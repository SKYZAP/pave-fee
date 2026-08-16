package fees

import (
	"context"
	"errors"
	"time"

	"encore.app/fees/config"
	"encore.app/fees/domain"
	"encore.dev/beta/errs"
	"github.com/google/uuid"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

type ListBillsRequest struct {
	OwnerID string `query:"owner_id"`
}

type ListBillsResponse struct {
	Bills []Bill `json:"bills"`
}

//encore:api public method=GET path=/v1/bills
func (s *Service) ListBills(ctx context.Context, request *ListBillsRequest) (*ListBillsResponse, error) {
	if request.OwnerID == "" {
		return nil, apiError(errs.InvalidArgument, "owner_id is required")
	}
	bills, err := s.repo.ListBills(ctx, request.OwnerID)
	if err != nil {
		return nil, mapAPIError(err)
	}
	response := make([]Bill, len(bills))
	for i, bill := range bills {
		response[i] = billFromDomain(bill)
	}
	return &ListBillsResponse{Bills: response}, nil
}

//encore:api public method=POST path=/v1/bills
func (s *Service) CreateBill(ctx context.Context, request *CreateBillRequest) (*Bill, error) {
	if err := ValidateIdempotencyKey(request.IdempotencyKey); err != nil {
		return nil, apiError(errs.InvalidArgument, err.Error())
	}
	if err := ValidateCreateBill(*request); err != nil {
		return nil, apiError(errs.InvalidArgument, err.Error())
	}
	hash, err := CanonicalHash(struct {
		OwnerID     string
		Currency    Currency
		PeriodStart string
		PeriodEnd   string
	}{request.OwnerID, request.Currency, request.PeriodStart.UTC().Format(timeFormat), request.PeriodEnd.UTC().Format(timeFormat)})
	if err != nil {
		return nil, apiError(errs.Internal, "failed to hash request")
	}
	billID := uuid.New()
	workflowID := workflowIDForBill(billID.String())
	bill, err := s.repo.CreateBill(ctx, CreateBillRecord{
		ID:      billID,
		OwnerID: request.OwnerID, Currency: request.Currency, PeriodStart: request.PeriodStart,
		PeriodEnd: request.PeriodEnd, IdempotencyKey: request.IdempotencyKey,
		RequestHash: hash, WorkflowID: workflowID,
	})
	if err != nil {
		return nil, mapAPIError(err)
	}
	if bill.ID != billID.String() {
		billID, err = uuid.Parse(bill.ID)
		if err != nil {
			return nil, apiError(errs.Internal, "stored bill has invalid id")
		}
		workflowID = bill.WorkflowID
	}
	_, err = s.temporal.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                                       workflowID,
		TaskQueue:                                currentTaskQueue(),
		WorkflowExecutionErrorWhenAlreadyStarted: false,
	}, BillWorkflow, WorkflowInput{
		BillID: billID.String(), OwnerID: request.OwnerID, WorkflowID: workflowID,
	})
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if !errors.As(err, &alreadyStarted) {
			return nil, apiError(errs.Unavailable, "workflow unavailable")
		}
	}
	response := billFromDomain(bill)
	return &response, nil
}

//encore:api public method=POST path=/v1/bills/:billID/items
func (s *Service) AddLineItem(ctx context.Context, billID string, request *AddLineItemRequest) (*LineItem, error) {
	request.BillID = billID
	if err := ValidateIdempotencyKey(request.IdempotencyKey); err != nil {
		return nil, apiError(errs.InvalidArgument, err.Error())
	}
	if err := ValidateAddLineItem(*request); err != nil {
		return nil, apiError(errs.InvalidArgument, err.Error())
	}
	if _, err := uuid.Parse(request.BillID); err != nil {
		return nil, apiError(errs.InvalidArgument, "invalid bill id")
	}
	hash, err := CanonicalHash(struct {
		Description string
		Currency    Currency
		Amount      int64
		Source      string
	}{request.Description, request.Currency, request.Amount, request.Source})
	if err != nil {
		return nil, apiError(errs.Internal, "failed to hash request")
	}
	handle, err := s.temporal.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID: workflowIDForBill(request.BillID),
		UpdateID:   request.IdempotencyKey,
		UpdateName: "AddLineItem",
		Args: []interface{}{AddLineItemCommand{
			TransactionID: request.IdempotencyKey, RequestHash: hash,
			Description: request.Description, Currency: request.Currency,
			Amount: request.Amount, Source: request.Source,
		}},
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})
	if err != nil {
		if billID, parseErr := uuid.Parse(request.BillID); parseErr == nil {
			if bill, getErr := s.repo.GetBill(ctx, billID); getErr == nil && bill.Status == StatusClosed {
				return nil, apiError(errs.FailedPrecondition, "bill_closed")
			}
		}
		return nil, mapAPIError(err)
	}
	var item LineItem
	if err := handle.Get(ctx, &item); err != nil {
		return nil, mapAPIError(err)
	}
	return &item, nil
}

//encore:api public method=GET path=/v1/bills/:billID
func (s *Service) GetBill(ctx context.Context, billID string) (*Bill, error) {
	id, err := uuid.Parse(billID)
	if err != nil {
		return nil, apiError(errs.InvalidArgument, "invalid bill id")
	}
	bill, err := s.repo.GetBill(ctx, id)
	if err != nil {
		return nil, mapAPIError(err)
	}
	response := billFromDomain(bill)
	return &response, nil
}

//encore:api public method=POST path=/v1/bills/:billID/close
func (s *Service) CloseBill(ctx context.Context, billID string, request *CloseBillRequest) (*Invoice, error) {
	request.BillID = billID
	if err := ValidateIdempotencyKey(request.IdempotencyKey); err != nil {
		return nil, apiError(errs.InvalidArgument, err.Error())
	}
	if _, err := uuid.Parse(request.BillID); err != nil {
		return nil, apiError(errs.InvalidArgument, "invalid bill id")
	}
	handle, err := s.temporal.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   workflowIDForBill(request.BillID),
		UpdateID:     request.IdempotencyKey,
		UpdateName:   "CloseBill",
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})
	if err != nil {
		if billID, parseErr := uuid.Parse(request.BillID); parseErr == nil {
			if bill, getErr := s.repo.GetBill(ctx, billID); getErr == nil && bill.Status == StatusClosed {
				return invoiceFromBill(bill), nil
			}
		}
		return nil, mapAPIError(err)
	}
	var invoice Invoice
	if err := handle.Get(ctx, &invoice); err != nil {
		return nil, mapAPIError(err)
	}
	return &invoice, nil
}

const timeFormat = "2006-01-02T15:04:05.999999999Z07:00"

func currentTaskQueue() string {
	return config.Load().TemporalTaskQueue
}

func invoiceFromBill(bill domain.Bill) *Invoice {
	closedAt := time.Time{}
	if bill.ClosedAt != nil {
		closedAt = *bill.ClosedAt
	}
	return &Invoice{
		ID: bill.ID, Status: bill.Status, Currency: bill.Currency,
		Total: moneyFromDomain(bill.Total), LineItems: lineItemsFromDomain(bill.LineItems), ClosedAt: closedAt, Version: bill.Version,
	}
}

func apiError(code errs.ErrCode, message string) error {
	return &errs.Error{Code: code, Message: message}
}

func mapAPIError(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return apiError(errs.NotFound, ErrNotFound.Error())
	case errors.Is(err, ErrBillClosed):
		return apiError(errs.FailedPrecondition, "bill_closed")
	case errors.Is(err, ErrConflict):
		return apiError(errs.Aborted, ErrConflict.Error())
	case errors.Is(err, ErrInvalidArgument):
		return apiError(errs.InvalidArgument, err.Error())
	case errors.Is(err, ErrUnsupportedCurrency):
		return apiError(errs.InvalidArgument, ErrUnsupportedCurrency.Error())
	case errors.Is(err, ErrCurrencyMismatch):
		return apiError(errs.InvalidArgument, ErrCurrencyMismatch.Error())
	default:
		return apiError(errs.Internal, "fees operation failed")
	}
}
