"use client";

export function UsageChart({
  title,
  used,
  total,
  unit,
}: {
  title: string;
  used: number;
  total: number;
  unit: string;
}) {
  const pct = total > 0 ? Math.min((used / total) * 100, 100) : 0;
  const isWarning = pct > 80;
  const isCritical = pct > 95;

  return (
    <div className="rounded-xl border border-white/[0.04] bg-white/[0.02] p-6 transition-colors duration-300 hover:border-white/[0.08]">
      <div className="flex items-center justify-between">
        <span className="text-[13px] text-white/30">{title}</span>
        <span className="text-[13px] text-white/50">
          {used.toLocaleString()} / {total.toLocaleString()} {unit}
        </span>
      </div>
      <div className="mt-4 h-1 overflow-hidden rounded-full bg-white/[0.04]">
        <div
          className={`h-full rounded-full transition-all duration-500 ${
            isCritical
              ? "bg-red-500/80"
              : isWarning
                ? "bg-amber-400/70"
                : "bg-strand/70"
          }`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <div className="mt-2 text-right text-[11px] text-white/20">
        {pct.toFixed(1)}% used
      </div>
    </div>
  );
}
