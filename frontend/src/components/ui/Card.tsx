import type { HTMLAttributes } from "react";

export function Card({ className = "", ...props }: HTMLAttributes<HTMLDivElement>) {
  return <section {...props} className={`rounded-2xl border border-slate-200 bg-white p-6 shadow-sm ${className}`} />;
}
