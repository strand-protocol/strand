import { PlanBadge } from "@/components/dashboard/plan-badge";

export default function SettingsPage() {
  return (
    <>
      <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">Settings</h1>
      <p className="mt-1 text-[13px] text-white/35">
        Manage your organisation settings
      </p>

      {/* Organisation info */}
      <div className="mt-8 rounded-xl border border-white/[0.04] bg-white/[0.02] p-6">
        <h2 className="text-sm font-semibold text-white/70">
          Organisation
        </h2>
        <div className="mt-4 space-y-4">
          {[
            { label: "Name", value: "Acme AI Corp" },
            { label: "Slug", value: "acme-ai" },
            { label: "Plan", value: "pro", badge: true },
            { label: "Status", value: "Active" },
            { label: "Tenant ID", value: "t_a1b2c3d4e5f6" },
          ].map((item) => (
            <div
              key={item.label}
              className="flex items-center justify-between text-sm"
            >
              <span className="text-white/25">{item.label}</span>
              {item.badge ? (
                <PlanBadge plan={item.value} />
              ) : (
                <span className="font-mono text-white/90">
                  {item.value}
                </span>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Danger zone */}
      <div className="mt-8 rounded-xl border border-red-500/30 bg-red-500/5 p-6">
        <h2 className="text-sm font-semibold text-red-400">Danger Zone</h2>
        <p className="mt-2 text-sm text-white/25">
          Permanently delete this organisation and all associated data. This
          action cannot be undone.
        </p>
        <button className="mt-4 rounded-full border border-red-500/50 px-4 py-2 text-sm text-red-400 transition-colors hover:bg-red-500/10">
          Delete Organisation
        </button>
      </div>
    </>
  );
}
