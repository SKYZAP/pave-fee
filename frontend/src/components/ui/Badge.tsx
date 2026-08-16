export function Badge({ status }: { status: "OPEN" | "CLOSED" }) {
  return (
    <span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${status === "OPEN" ? "bg-emerald-100 text-emerald-700" : "bg-slate-100 text-slate-700"}`}>
      {status}
    </span>
  );
}
