import { statusColor } from "@/lib/format";
import { PlanBadge } from "@/components/dashboard/plan-badge";

const demoClusters = [
  {
    id: "prod-cluster-1",
    name: "Production US West",
    region: "us-west-2",
    status: "running",
    nodeCount: 8,
    plan: "pro",
  },
  {
    id: "staging-cluster-1",
    name: "Staging",
    region: "us-east-1",
    status: "running",
    nodeCount: 3,
    plan: "pro",
  },
];

export default function ClustersPage() {
  return (
    <>
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">Clusters</h1>
          <p className="mt-1 text-[13px] text-white/35">
            Managed Strand Protocol clusters
          </p>
        </div>
      </div>

      <div className="mt-6 grid gap-3 md:grid-cols-2">
        {demoClusters.map((c) => (
          <div
            key={c.id}
            className="rounded-xl border border-white/[0.04] bg-white/[0.02] p-6 transition-colors hover:border-white/[0.08]"
          >
            <div className="flex items-center justify-between">
              <h3 className="font-semibold text-white/90">{c.name}</h3>
              <PlanBadge plan={c.plan} />
            </div>
            <div className="mt-4 grid grid-cols-3 gap-3 text-sm">
              <div>
                <div className="text-[13px] text-white/25">Region</div>
                <div className="font-mono text-white/90">{c.region}</div>
              </div>
              <div>
                <div className="text-[13px] text-white/25">Nodes</div>
                <div className="text-white/90">{c.nodeCount}</div>
              </div>
              <div>
                <div className="text-[13px] text-white/25">Status</div>
                <div className={`font-medium ${statusColor(c.status)}`}>
                  {c.status}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </>
  );
}
