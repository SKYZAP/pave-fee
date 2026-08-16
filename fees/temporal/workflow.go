package temporal

import (
	"context"
	"errors"
	"time"

	"encore.app/fees/database"
	"encore.app/fees/domain"
	"github.com/google/uuid"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var RetryPolicy = sdktemporal.RetryPolicy{
	InitialInterval:    time.Second,
	BackoffCoefficient: 2,
	MaximumInterval:    10 * time.Second,
	MaximumAttempts:    5,
	NonRetryableErrorTypes: []string{
		"fees.business",
	},
}

type WorkflowInput struct {
	BillID     string
	OwnerID    string
	WorkflowID string
}

type AddLineItemCommand struct {
	TransactionID string
	RequestHash   string
	Description   string
	Currency      domain.Currency
	Amount        int64
	Source        string
}

type AppendLineItemInput struct {
	BillID  string
	Command AddLineItemCommand
}

type CloseBillInput struct {
	BillID string
}

func WorkflowIDForBill(id string) string {
	return "bill-" + id
}

func BillWorkflow(ctx workflow.Context, input WorkflowInput) (domain.Invoice, error) {
	activityOptions := workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &RetryPolicy}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)
	var activities *Activities
	closed := workflow.NewChannel(ctx)

	if err := workflow.SetUpdateHandler(ctx, "AddLineItem", func(ctx workflow.Context, command AddLineItemCommand) (domain.LineItem, error) {
		var item domain.LineItem
		err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, activityOptions), activities.AppendLineItem, AppendLineItemInput{
			BillID: input.BillID, Command: command,
		}).Get(ctx, &item)
		return item, err
	}); err != nil {
		return domain.Invoice{}, err
	}
	if err := workflow.SetUpdateHandler(ctx, "CloseBill", func(ctx workflow.Context) (domain.Invoice, error) {
		var invoice domain.Invoice
		err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, activityOptions), activities.CloseBill, CloseBillInput{
			BillID: input.BillID,
		}).Get(ctx, &invoice)
		if err == nil {
			closed.Send(ctx, invoice)
		}
		return invoice, err
	}); err != nil {
		return domain.Invoice{}, err
	}

	var projection domain.Bill
	if err := workflow.ExecuteActivity(ctx, activities.CreateBillProjection, input).Get(ctx, &projection); err != nil {
		return domain.Invoice{}, err
	}
	timer := workflow.NewTimer(ctx, projection.PeriodEnd.Sub(workflow.Now(ctx)))
	var invoice domain.Invoice
	var automaticCloseErr error
	workflow.NewSelector(ctx).
		AddReceive(closed, func(channel workflow.ReceiveChannel, more bool) { channel.Receive(ctx, &invoice) }).
		AddFuture(timer, func(workflow.Future) {
			automaticCloseErr = workflow.ExecuteActivity(ctx, activities.CloseBill, CloseBillInput{
				BillID: input.BillID,
			}).Get(ctx, &invoice)
		}).Select(ctx)
	if automaticCloseErr != nil {
		return domain.Invoice{}, automaticCloseErr
	}
	return invoice, nil
}

type Activities struct {
	Repository *database.Repository
}

func (a *Activities) CreateBillProjection(ctx context.Context, input WorkflowInput) (domain.Bill, error) {
	billID, err := uuid.Parse(input.BillID)
	if err != nil {
		return domain.Bill{}, sdktemporal.NewNonRetryableApplicationError("invalid bill id", "fees.business", err)
	}
	bill, err := a.Repository.GetBill(ctx, billID)
	if err != nil {
		return domain.Bill{}, err
	}
	if bill.WorkflowID != input.WorkflowID || bill.OwnerID != input.OwnerID {
		return domain.Bill{}, sdktemporal.NewNonRetryableApplicationError("bill projection mismatch", "fees.business", nil)
	}
	return bill, nil
}

func (a *Activities) AppendLineItem(ctx context.Context, input AppendLineItemInput) (domain.LineItem, error) {
	billID, err := uuid.Parse(input.BillID)
	if err != nil {
		return domain.LineItem{}, sdktemporal.NewNonRetryableApplicationError("invalid bill id", "fees.business", err)
	}
	item, err := a.Repository.AppendLineItem(ctx, database.AppendLineItemRecord{
		BillID: billID, TransactionID: input.Command.TransactionID, RequestHash: input.Command.RequestHash,
		Description: input.Command.Description, Currency: input.Command.Currency, Amount: input.Command.Amount, Source: input.Command.Source,
	})
	if err != nil {
		return domain.LineItem{}, activityError(err)
	}
	return item, nil
}

func (a *Activities) CloseBill(ctx context.Context, input CloseBillInput) (domain.Invoice, error) {
	billID, err := uuid.Parse(input.BillID)
	if err != nil {
		return domain.Invoice{}, sdktemporal.NewNonRetryableApplicationError("invalid bill id", "fees.business", err)
	}
	invoice, err := a.Repository.CloseBill(ctx, billID)
	if err != nil {
		return domain.Invoice{}, activityError(err)
	}
	return invoice, nil
}

func activityError(err error) error {
	if errors.Is(err, domain.ErrBillClosed) || errors.Is(err, domain.ErrConflict) ||
		errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidArgument) ||
		errors.Is(err, domain.ErrUnsupportedCurrency) || errors.Is(err, domain.ErrCurrencyMismatch) {
		return sdktemporal.NewNonRetryableApplicationError(err.Error(), "fees.business", err)
	}
	return err
}
