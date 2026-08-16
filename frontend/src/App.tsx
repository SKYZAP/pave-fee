import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { feesAPI } from "./api/fees";
import { retainIdempotencyKey } from "./api/idempotency";
import { errorMessage } from "./lib/errors";
import { formatMoney } from "./lib/money";
import type { Bill, Currency } from "./types/fees";
import { Badge } from "./components/ui/Badge";
import { Button } from "./components/ui/Button";
import { Card } from "./components/ui/Card";

const ownerID = "merchant_123";

export default function App() {
  const [path, setPath] = useState(window.location.pathname);
  const navigate = (next: string) => {
    window.history.pushState({}, "", next);
    setPath(next);
  };

  if (path === "/bills/new") return <Shell><CreateBillPage navigate={navigate} /></Shell>;
  const invoice = path.match(/^\/bills\/([^/]+)\/invoice$/);
  if (invoice) return <Shell><BillPage id={invoice[1]} invoice navigate={navigate} /></Shell>;
  const details = path.match(/^\/bills\/([^/]+)$/);
  if (details) return <Shell><BillPage id={details[1]} navigate={navigate} /></Shell>;
  return <Shell><Dashboard navigate={navigate} /></Shell>;
}

function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-slate-50 text-slate-900">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-5">
          <button className="text-xl font-bold tracking-tight" onClick={() => window.location.assign("/bills")}>Fees Ledger</button>
          <span className="text-sm text-slate-500">{ownerID}</span>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-6 py-10">{children}</main>
    </div>
  );
}

function Dashboard({ navigate }: { navigate: (path: string) => void }) {
  const query = useQuery({ queryKey: ["bills"], queryFn: () => feesAPI.listBills(ownerID) });
  const [filter, setFilter] = useState<"ALL" | "OPEN" | "CLOSED">("ALL");
  const bills = (query.data?.bills ?? []).filter((bill) => filter === "ALL" || bill.status === filter);
  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div><p className="text-sm font-semibold uppercase tracking-widest text-indigo-600">Billing workspace</p><h1 className="mt-2 text-4xl font-bold tracking-tight">Bills</h1><p className="mt-2 text-slate-500">Immutable invoices in their bill currency.</p></div>
        <Button onClick={() => navigate("/bills/new")}>Create bill</Button>
      </div>
      {query.isLoading && <Card>Loading bills…</Card>}
      {query.error && <Card className="border-rose-200 text-rose-700">{errorMessage(query.error)}</Card>}
      {!query.isLoading && !query.error && <Card>
        <div className="mb-6 flex gap-2">{(["ALL", "OPEN", "CLOSED"] as const).map((value) => <button key={value} onClick={() => setFilter(value)} className={`rounded-full px-3 py-1 text-sm ${filter === value ? "bg-slate-900 text-white" : "bg-slate-100 text-slate-600"}`}>{value}</button>)}</div>
        {bills.length === 0 ? <p className="py-12 text-center text-slate-400">No bills match this filter.</p> : <div className="divide-y divide-slate-100">{bills.map((bill) => <BillRow key={bill.id} bill={bill} navigate={navigate} />)}</div>}
      </Card>}
    </div>
  );
}

function BillRow({ bill, navigate }: { bill: Bill; navigate: (path: string) => void }) {
  return <button onClick={() => navigate(`/bills/${bill.id}`)} className="flex w-full items-center justify-between gap-4 py-4 text-left hover:bg-slate-50">
    <div><div className="flex items-center gap-3 font-semibold"><span>{new Date(bill.period_start).toLocaleDateString()} – {new Date(bill.period_end).toLocaleDateString()}</span><Badge status={bill.status} /></div><p className="mt-1 text-sm text-slate-500">{bill.line_items?.length ?? 0} {bill.currency} line items</p></div>
    <Totals total={bill.total ?? []} />
  </button>;
}

function CreateBillPage({ navigate }: { navigate: (path: string) => void }) {
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [currency, setCurrency] = useState<Currency>("USD");
  const mutation = useMutation({
    mutationFn: () => feesAPI.createBill(
      { owner_id: ownerID, currency, period_start: new Date(start).toISOString(), period_end: new Date(end).toISOString() },
      retainIdempotencyKey(undefined),
    ),
    onSuccess: (bill) => navigate(`/bills/${bill.id}`),
  });
  const invalid = !start || !end || new Date(end) <= new Date(start);
  return <div className="mx-auto max-w-xl space-y-6"><div><p className="text-sm font-semibold uppercase tracking-widest text-indigo-600">New billing period</p><h1 className="mt-2 text-4xl font-bold">Create a bill</h1></div><Card><form className="space-y-5" onSubmit={(event) => { event.preventDefault(); mutation.mutate(); }}><label className="block text-sm font-medium">Bill currency<select value={currency} onChange={(event) => setCurrency(event.target.value as Currency)} className="mt-2 w-full rounded-lg border-slate-300"><option value="USD">USD</option><option value="GEL">GEL</option></select></label><label className="block text-sm font-medium">Period start<input required type="datetime-local" value={start} onChange={(event) => setStart(event.target.value)} className="mt-2 w-full rounded-lg border-slate-300" /></label><label className="block text-sm font-medium">Period end<input required type="datetime-local" value={end} onChange={(event) => setEnd(event.target.value)} className="mt-2 w-full rounded-lg border-slate-300" /></label>{invalid && start && end && <p className="text-sm text-rose-600">The period end must be after the start.</p>}{mutation.error && <p className="text-sm text-rose-600">{errorMessage(mutation.error)}</p>}<div className="flex gap-3"><Button type="button" className="!bg-red-600 !text-white hover:!bg-red-700" onClick={() => navigate("/bills")}>Cancel</Button><Button disabled={invalid || mutation.isPending}>{mutation.isPending ? "Creating…" : "Create bill"}</Button></div></form></Card></div>;
}

function BillPage({ id, invoice = false, navigate }: { id: string; invoice?: boolean; navigate: (path: string) => void }) {
  const queryClient = useQueryClient();
  const query = useQuery({ queryKey: ["bill", id], queryFn: () => feesAPI.getBill(id) });
  const [description, setDescription] = useState("");
  const [amount, setAmount] = useState("");
  const add = useMutation({ mutationFn: () => feesAPI.addLineItem(id, { description, currency: query.data?.currency ?? "USD", amount: Number(amount), source: "fees-ui" }, retainIdempotencyKey(undefined)), onSuccess: () => { setDescription(""); setAmount(""); queryClient.invalidateQueries({ queryKey: ["bill", id] }); } });
  const close = useMutation({ mutationFn: () => feesAPI.closeBill(id, retainIdempotencyKey(undefined)), onSuccess: () => queryClient.invalidateQueries({ queryKey: ["bill", id] }) });
  if (query.isLoading) return <Card>Loading bill…</Card>;
  if (query.error || !query.data) return <Card className="text-rose-700">{errorMessage(query.error)}</Card>;
  const bill = query.data;
  const lineItems = bill.line_items ?? [];
  const itemCurrency = bill.currency;
  return <div className="space-y-8"><div className="flex flex-wrap items-start justify-between gap-4"><div><button className="text-sm text-indigo-600" onClick={() => navigate("/bills")}>← All bills</button><h1 className="mt-3 text-4xl font-bold">{invoice ? "Invoice" : "Bill details"}</h1><p className="mt-2 text-slate-500">{new Date(bill.period_start).toLocaleString()} – {new Date(bill.period_end).toLocaleString()}</p></div><Badge status={bill.status} /></div><Card><div className="flex items-center justify-between"><h2 className="text-lg font-semibold">Bill total ({bill.currency})</h2><Totals total={bill.total ?? []} /></div></Card><Card><h2 className="mb-4 text-lg font-semibold">{invoice ? "Charged line items" : "Line items"}</h2><div className="divide-y divide-slate-100">{lineItems.map((item) => <div key={item.id} className="flex items-center justify-between py-3"><div><p className="font-medium">{item.description}</p><p className="text-xs text-slate-400">{item.transaction_id} · {item.source}</p></div><span className="font-semibold">{formatMoney({ currency: item.currency, amount: item.amount })}</span></div>)}{lineItems.length === 0 && <p className="py-8 text-center text-slate-400">No line items yet.</p>}</div></Card>{!invoice && bill.status === "OPEN" && <Card><h2 className="mb-4 text-lg font-semibold">Add line item</h2><form className="grid gap-4 md:grid-cols-[1fr_8rem_auto]" onSubmit={(event) => { event.preventDefault(); add.mutate(); }}><input required placeholder="Description" value={description} onChange={(event) => setDescription(event.target.value)} className="rounded-lg border-slate-300" /><input required min="1" type="number" placeholder={`${itemCurrency} minor units`} value={amount} onChange={(event) => setAmount(event.target.value)} className="rounded-lg border-slate-300" /><Button disabled={add.isPending}>{add.isPending ? "Adding…" : "Add item"}</Button></form>{add.error && <p className="mt-3 text-sm text-rose-600">{errorMessage(add.error)}</p>}<div className="mt-6 flex justify-end"><Button className="bg-slate-900 hover:bg-slate-700" disabled={close.isPending} onClick={() => { if (window.confirm("Close this bill permanently?")) close.mutate(); }}>{close.isPending ? "Closing…" : "Close bill"}</Button></div></Card>}{bill.status === "CLOSED" && !invoice && <Button onClick={() => navigate(`/bills/${id}/invoice`)}>View immutable invoice</Button>}</div>;
}

function Totals({ total }: { total: Bill["total"] }) {
  return <div className="flex gap-4">{(total ?? []).map((money) => <span key={money.currency} className="text-sm font-semibold text-slate-600">{formatMoney(money)}</span>)}</div>;
}
