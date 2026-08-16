package temporal

import (
	"errors"
	"testing"
	"time"

	"encore.app/fees/domain"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestWorkflowIDForBillIsDeterministic(t *testing.T) {
	const billID = "b26816cc-34fe-4d49-bc59-4bb3cf75f509"
	if got := WorkflowIDForBill(billID); got != "bill-"+billID {
		t.Fatalf("workflow id = %q", got)
	}
}

func TestBillWorkflowPropagatesAutomaticCloseFailure(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	activities := &Activities{}
	periodEnd := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)

	env.OnActivity(activities.CreateBillProjection, mock.Anything, mock.Anything).
		Return(domain.Bill{PeriodEnd: periodEnd}, nil)
	env.OnActivity(activities.CloseBill, mock.Anything, mock.Anything).
		Return(domain.Invoice{}, errors.New("database unavailable"))

	env.SetStartTime(periodEnd.Add(-time.Hour))
	env.ExecuteWorkflow(BillWorkflow, WorkflowInput{
		BillID: "00000000-0000-0000-0000-000000000001", OwnerID: "owner",
		WorkflowID: "bill-00000000-0000-0000-0000-000000000001",
	})
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("expected automatic close failure to fail the workflow")
	}
}

func TestBillWorkflowAutomaticCloseReturnsBillCurrencyInvoice(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	activities := &Activities{}
	periodEnd := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)

	env.OnActivity(activities.CreateBillProjection, mock.Anything, mock.Anything).
		Return(domain.Bill{PeriodEnd: periodEnd}, nil)
	env.OnActivity(activities.CloseBill, mock.Anything, mock.Anything).
		Return(domain.Invoice{
			ID: "bill-1", Status: domain.StatusClosed, Currency: domain.CurrencyUSD,
			Total: []domain.Money{{Currency: domain.CurrencyUSD, Amount: 2000}}, ClosedAt: periodEnd, Version: 3,
		}, nil)

	env.SetStartTime(periodEnd.Add(-time.Hour))
	env.ExecuteWorkflow(BillWorkflow, WorkflowInput{
		BillID: "00000000-0000-0000-0000-000000000001", OwnerID: "owner",
		WorkflowID: "bill-00000000-0000-0000-0000-000000000001",
	})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var invoice domain.Invoice
	if err := env.GetWorkflowResult(&invoice); err != nil {
		t.Fatal(err)
	}
	if invoice.Currency != domain.CurrencyUSD || invoice.Total[0].Amount != 2000 {
		t.Fatalf("unexpected invoice: %#v", invoice)
	}
}
