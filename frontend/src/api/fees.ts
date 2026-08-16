import type {
  AddLineItemInput,
  Bill,
  CreateBillInput,
  LineItem,
} from "../types/fees";
import { retainIdempotencyKey } from "./idempotency";

const baseURL = window.location.origin;

export class FeesAPIError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(
  path: string,
  init: RequestInit = {},
  key?: string,
): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");
  if (key) headers.set("Idempotency-Key", key);
  const response = await fetch(`${baseURL}${path}`, { ...init, headers });
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as {
      code?: string;
      message?: string;
    };
    throw new FeesAPIError(
      response.status,
      body.code ?? "unknown",
      body.message ?? `Request failed with status ${response.status}`,
    );
  }
  return (await response.json()) as T;
}

export const feesAPI = {
  listBills(ownerID: string): Promise<{ bills: Bill[] }> {
    return request(`/v1/bills?owner_id=${encodeURIComponent(ownerID)}`);
  },
  getBill(id: string): Promise<Bill> {
    return request(`/v1/bills/${encodeURIComponent(id)}`);
  },
  createBill(input: CreateBillInput, key?: string): Promise<Bill> {
    const stableKey = retainIdempotencyKey(key);
    return request("/v1/bills", {
      method: "POST",
      body: JSON.stringify(input),
    }, stableKey);
  },
  addLineItem(id: string, input: AddLineItemInput, key?: string): Promise<LineItem> {
    const stableKey = retainIdempotencyKey(key);
    return request(`/v1/bills/${encodeURIComponent(id)}/items`, {
      method: "POST",
      body: JSON.stringify(input),
    }, stableKey);
  },
  closeBill(id: string, key?: string): Promise<Bill> {
    const stableKey = retainIdempotencyKey(key);
    return request(`/v1/bills/${encodeURIComponent(id)}/close`, {
      method: "POST",
      body: JSON.stringify({}),
    }, stableKey);
  },
};
