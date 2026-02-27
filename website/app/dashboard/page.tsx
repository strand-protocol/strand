import { Server, GitBranch, Shield, Activity } from "lucide-react";
import { StatCard } from "@/components/dashboard/stat-card";
import { UsageChart } from "@/components/dashboard/usage-chart";

export default function DashboardPage() {
  return (
    <>
      <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">Dashboard</h1>
      <p className="mt-1 text-[13px] text-white/35">
        Overview of your Strand Protocol deployment
      </p>

      {/* Stats */}
      <div className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Nodes" value="12" change="+3" icon={Server} />
        <StatCard title="Clusters" value="2" icon={GitBranch} />
        <StatCard title="Active MICs" value="847" change="+120" icon={Shield} />
        <StatCard
          title="Traffic (30d)"
          value="42.8 GB"
          change="+12%"
          icon={Activity}
        />
      </div>

      {/* Usage */}
      <div className="mt-6 grid gap-3 md:grid-cols-2">
        <UsageChart title="MICs this month" used={847} total={10000} unit="MICs" />
        <UsageChart
          title="Traffic this month"
          used={42.8}
          total={100}
          unit="GB"
        />
      </div>

      {/* Recent activity */}
      <div className="mt-6 rounded-xl border border-white/[0.04] bg-white/[0.02] p-6">
        <h2 className="text-sm font-semibold text-white/70">
          Recent Activity
        </h2>
        <div className="mt-4 space-y-3">
          {[
            { action: "Node registered", detail: "node-gpu-a100-03", time: "2m ago" },
            { action: "MIC issued", detail: "for llama-70b deployment", time: "15m ago" },
            { action: "Route updated", detail: "inference-pool-us-west", time: "1h ago" },
            { action: "Cluster scaled", detail: "prod-cluster-1 → 8 nodes", time: "3h ago" },
          ].map((item) => (
            <div
              key={item.detail}
              className="flex items-center justify-between text-[13px]"
            >
              <div>
                <span className="text-white/60">{item.action}</span>{" "}
                <span className="text-white/25">{item.detail}</span>
              </div>
              <span className="text-white/20">{item.time}</span>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
