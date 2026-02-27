import { UsageChart } from "@/components/dashboard/usage-chart";

export default function TrafficPage() {
  return (
    <>
      <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">Traffic</h1>
      <p className="mt-1 text-[13px] text-white/35">
        Network traffic and bandwidth usage across your clusters
      </p>

      <div className="mt-8 grid gap-3 md:grid-cols-2">
        <UsageChart title="Traffic this month" used={42.8} total={100} unit="GB" />
        <UsageChart title="MICs this month" used={847} total={10000} unit="MICs" />
      </div>

      {/* Per-cluster breakdown */}
      <div className="mt-8 rounded-xl border border-white/[0.04] bg-white/[0.02] p-6">
        <h2 className="text-sm font-semibold text-white/70">
          Traffic by Cluster
        </h2>
        <div className="mt-4 space-y-4">
          {[
            { name: "prod-cluster-1", ingress: "28.4 GB", egress: "12.1 GB" },
            { name: "staging-cluster-1", ingress: "1.8 GB", egress: "0.5 GB" },
          ].map((c) => (
            <div
              key={c.name}
              className="flex items-center justify-between text-sm"
            >
              <span className="font-mono text-white/90">{c.name}</span>
              <div className="flex gap-6">
                <div>
                  <span className="text-white/25">In: </span>
                  <span className="text-white/40">{c.ingress}</span>
                </div>
                <div>
                  <span className="text-white/25">Out: </span>
                  <span className="text-white/40">{c.egress}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
