import type { LucideIcon } from "lucide-react";

export function StatCard({
  title,
  value,
  change,
  icon: Icon,
}: {
  title: string;
  value: string;
  change?: string;
  icon: LucideIcon;
}) {
  const isPositive = change?.startsWith("+");
  return (
    <div className="rounded-xl border border-white/[0.04] bg-white/[0.02] p-6 transition-colors duration-300 hover:border-white/[0.08]">
      <div className="flex items-center justify-between">
        <span className="text-[13px] text-white/30">{title}</span>
        <Icon className="h-4 w-4 text-white/15" />
      </div>
      <div className="mt-3 text-2xl font-bold tracking-[-0.02em] text-white/90">{value}</div>
      {change && (
        <span
          className={`mt-1.5 inline-block text-[13px] ${
            isPositive ? "text-emerald-400/70" : "text-red-400/70"
          }`}
        >
          {change} from last month
        </span>
      )}
    </div>
  );
}
