import { PlanBadge } from "@/components/dashboard/plan-badge";
import { UsageChart } from "@/components/dashboard/usage-chart";

export default function BillingPage() {
  return (
    <>
      <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">Billing</h1>
      <p className="mt-1 text-[13px] text-white/35">
        Plan details, usage, and invoices
      </p>

      {/* Current plan */}
      <div className="mt-8 rounded-xl border border-white/[0.04] bg-white/[0.02] p-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <h2 className="text-sm font-semibold text-white/70">
              Current Plan
            </h2>
            <PlanBadge plan="pro" />
          </div>
          <a
            href="/pricing"
            className="rounded-full border border-white/[0.08] px-4 py-2 text-sm text-white/50 transition-colors hover:border-white/[0.16] hover:text-white/90"
          >
            Upgrade Plan
          </a>
        </div>
        <div className="mt-4 grid gap-3 sm:grid-cols-3 text-sm">
          <div>
            <div className="text-[13px] text-white/25">Base Price</div>
            <div className="text-xl font-bold text-white/90">$5,000/mo</div>
          </div>
          <div>
            <div className="text-[13px] text-white/25">Current Period</div>
            <div className="text-white/90">Feb 1 — Feb 28, 2026</div>
          </div>
          <div>
            <div className="text-[13px] text-white/25">Estimated Total</div>
            <div className="text-xl font-bold text-white/90">$5,212</div>
          </div>
        </div>
      </div>

      {/* Usage meters */}
      <div className="mt-6 grid gap-3 md:grid-cols-3">
        <UsageChart title="MICs" used={847} total={10000} unit="MICs" />
        <UsageChart title="Traffic" used={42.8} total={100} unit="GB" />
        <UsageChart title="Node Hours" used={5760} total={10800} unit="hrs" />
      </div>

      {/* Invoice history */}
      <div className="mt-8 rounded-xl border border-white/[0.04] bg-white/[0.02] p-6">
        <h2 className="text-sm font-semibold text-white/70">
          Invoice History
        </h2>
        <div className="mt-4 space-y-3">
          {[
            { period: "January 2026", amount: "$5,089", status: "Paid" },
            { period: "December 2025", amount: "$5,156", status: "Paid" },
            { period: "November 2025", amount: "$5,000", status: "Paid" },
          ].map((inv) => (
            <div
              key={inv.period}
              className="flex items-center justify-between text-sm"
            >
              <span className="text-white/90">{inv.period}</span>
              <div className="flex items-center gap-4">
                <span className="text-white/40">{inv.amount}</span>
                <span className="rounded-full bg-green-500/10 px-2 py-0.5 text-xs text-green-400">
                  {inv.status}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
