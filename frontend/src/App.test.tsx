import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import App from "./App";

const bill = {
  id: "bill-1",
  owner_id: "merchant_123",
  period_start: "2026-08-01T00:00:00Z",
  period_end: "2026-09-01T00:00:00Z",
  status: "OPEN",
  currency: "USD",
  total: null,
  line_items: [],
  version: 0,
  workflow_id: "bill-bill-1",
};

const detailedBill = {
  ...bill,
  status: "CLOSED",
  total: [{ currency: "USD", amount: 2648 }],
  closed_at: "2026-09-01T00:00:03Z",
};

function renderApp() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <App />
    </QueryClientProvider>,
  );
}

describe("Fees app", () => {
  beforeEach(() => {
    window.history.pushState({}, "", "/bills");
    window.fetch = jest.fn().mockImplementation((input, init) => {
      const url = String(input);
      if (url.includes("/v1/bills?")) {
        return Promise.resolve(new Response(JSON.stringify({ bills: [bill] }), { status: 200 }));
      }
      if (url.endsWith("/v1/bills/bill-1")) {
        return Promise.resolve(new Response(JSON.stringify(detailedBill), { status: 200 }));
      }
      return Promise.resolve(new Response(JSON.stringify({ message: "not found" }), { status: 404 }));
    });
  });

  it("shows bills and separate currency-aware status", async () => {
    renderApp();
    expect(await screen.findByText("Bills")).toBeInTheDocument();
    expect((await screen.findAllByText("OPEN")).length).toBeGreaterThan(1);
  });

  it("navigates to bill creation", async () => {
    renderApp();
    await userEvent.click(await screen.findByText("Create bill"));
    expect(screen.getByText("Create a bill")).toBeInTheDocument();
  });

  it("renders an empty state when the API returns a null bill list", async () => {
    window.fetch = jest.fn().mockImplementation((input) => {
      const url = String(input);
      if (url.includes("/v1/bills?")) {
        return Promise.resolve(new Response(JSON.stringify({ bills: null }), { status: 200 }));
      }
      return Promise.resolve(new Response(JSON.stringify({ message: "not found" }), { status: 404 }));
    });

    renderApp();

    expect(await screen.findByText("No bills match this filter.")).toBeInTheDocument();
  });

  it("shows a single bill currency total in bill details", async () => {
    renderApp();
    await userEvent.click(await screen.findByText("0 USD line items"));
    expect(await screen.findByText("Bill total (USD)")).toBeInTheDocument();
    expect(screen.queryByText(/FX rate used/i)).not.toBeInTheDocument();
  });
});
