"use client";

export function PricingToggle({
  annual,
  onToggle,
}: {
  annual: boolean;
  onToggle: () => void;
}) {
  return (
    <div className="flex items-center justify-center gap-4">
      <span
        className={`text-sm transition-colors duration-300 ${
          !annual ? "text-white/80" : "text-white/25"
        }`}
      >
        Monthly
      </span>
      <button
        onClick={onToggle}
        className="relative h-7 w-12 rounded-full border border-white/[0.06] bg-white/[0.04] transition-colors duration-300"
        aria-label="Toggle annual billing"
      >
        <span
          className={`absolute top-0.5 h-6 w-6 rounded-full bg-strand transition-transform duration-300 ${
            annual ? "translate-x-5" : "translate-x-0.5"
          }`}
        />
      </button>
      <span
        className={`text-sm transition-colors duration-300 ${
          annual ? "text-white/80" : "text-white/25"
        }`}
      >
        Annual{" "}
        <span className="text-[11px] font-medium text-strand-300">Save 20%</span>
      </span>
    </div>
  );
}
