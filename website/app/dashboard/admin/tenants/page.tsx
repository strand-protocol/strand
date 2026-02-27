import { PlanBadge } from "@/components/dashboard/plan-badge";
import { statusColor } from "@/lib/format";

const demoTenants = [
  {
    id: "t_a1b2c3d4",
    name: "Acme AI Corp",
    slug: "acme-ai",
    plan: "pro",
    status: "active",
    clusters: 2,
    nodes: 11,
    mics: 847,
  },
  {
    id: "t_e5f6g7h8",
    name: "InferenceIO",
    slug: "inference-io",
    plan: "enterprise",
    status: "active",
    clusters: 5,
    nodes: 42,
    mics: 3200,
  },
  {
    id: "t_i9j0k1l2",
    name: "Dev Sandbox",
    slug: "dev-sandbox",
    plan: "free",
    status: "active",
    clusters: 1,
    nodes: 2,
    mics: 15,
  },
];

export default function AdminTenantsPage() {
  return (
    <>
      <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">
        Tenants (Admin)
      </h1>
      <p className="mt-1 text-[13px] text-white/35">
        System-wide tenant management
      </p>

      <div className="mt-6 overflow-x-auto rounded-xl border border-white/[0.04]">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-white/[0.04] bg-white/[0.02] text-[11px] uppercase tracking-[0.15em] text-white/25">
            <tr>
              <th className="px-4 py-3 font-medium">Tenant</th>
              <th className="px-4 py-3 font-medium">Plan</th>
              <th className="px-4 py-3 font-medium">Status</th>
              <th className="px-4 py-3 font-medium">Clusters</th>
              <th className="px-4 py-3 font-medium">Nodes</th>
              <th className="px-4 py-3 font-medium">MICs</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-white/[0.04]">
            {demoTenants.map((t) => (
              <tr
                key={t.id}
                className="transition-colors hover:bg-white/[0.02]"
              >
                <td className="px-4 py-3">
                  <div className="font-medium text-white/90">{t.name}</div>
                  <div className="font-mono text-xs text-white/25">
                    {t.slug}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <PlanBadge plan={t.plan} />
                </td>
                <td className="px-4 py-3">
                  <span className={`font-medium ${statusColor(t.status)}`}>
                    {t.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-white/40">{t.clusters}</td>
                <td className="px-4 py-3 text-white/40">{t.nodes}</td>
                <td className="px-4 py-3 text-white/40">
                  {t.mics.toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
