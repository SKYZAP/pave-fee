export type Currency = "GEL" | "USD";
export type BillStatus = "OPEN" | "CLOSED";

export interface Money {
  currency: Currency;
  amount: number;
}

export interface LineItem {
  id: string;
  bill_id: string;
  transaction_id: string;
  description: string;
  currency: Currency;
  amount: number;
  source: string;
  created_at: string;
}

export interface Bill {
  id: string;
  owner_id: string;
  period_start: string;
  period_end: string;
  status: BillStatus;
  currency: Currency;
  total: Money[] | null;
  line_items: LineItem[];
  closed_at?: string;
  version: number;
  workflow_id: string;
}

export interface CreateBillInput {
  owner_id: string;
  currency: Currency;
  period_start: string;
  period_end: string;
}

export interface AddLineItemInput {
  description: string;
  currency: Currency;
  amount: number;
  source?: string;
}

