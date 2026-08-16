import type { Money } from "../types/fees";

const currencyMeta = {
  GEL: { symbol: "₾", digits: 2 },
  USD: { symbol: "$", digits: 2 },
} as const;

export function formatMoney(money: Money): string {
  const meta = currencyMeta[money.currency];
  const absolute = Math.abs(money.amount).toString().padStart(meta.digits + 1, "0");
  const split = absolute.length - meta.digits;
  const sign = money.amount < 0 ? "-" : "";
  return `${sign}${meta.symbol}${absolute.slice(0, split)}.${absolute.slice(split)}`;
}
