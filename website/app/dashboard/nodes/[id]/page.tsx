import { Server, Activity, Clock, Cpu } from "lucide-react";
import { StatCard } from "@/components/dashboard/stat-card";

export default async function NodeDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <>
      <div className="flex items-center gap-3">
        <Server className="h-6 w-6 text-strand" />
        <div>
          <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">{id}</h1>
          <p className="text-[13px] text-white/35">Node details and metrics</p>
        </div>
      </div>

      <div className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Connections" value="24" icon={Activity} />
        <StatCard title="Avg Latency" value="1.2ms" icon={Clock} />
        <StatCard title="Traffic (24h)" value="3.2 GB" icon={Activity} />
        <StatCard title="Firmware" value="0.1.0" icon={Cpu} />
      </div>

      <div className="mt-8 rounded-xl border border-white/[0.04] bg-white/[0.02] p-6">
        <h2 className="text-sm font-semibold text-white/70">
          Node Configuration
        </h2>
        <div className="mt-4 grid gap-3 sm:grid-cols-2">
          {[
            { label: "Node ID", value: id },
            { label: "Address", value: "10.0.1.10:6477" },
            { label: "Status", value: "online" },
            { label: "Cluster", value: "prod-cluster-1" },
            { label: "Region", value: "us-west-2" },
            { label: "Last Seen", value: "just now" },
          ].map((item) => (
            <div key={item.label} className="flex justify-between text-sm">
              <span className="text-white/25">{item.label}</span>
              <span className="font-mono text-white/90">{item.value}</span>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
