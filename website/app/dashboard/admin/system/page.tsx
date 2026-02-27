import { Server, Database, Shield, Activity } from "lucide-react";
import { StatCard } from "@/components/dashboard/stat-card";

export default function AdminSystemPage() {
  return (
    <>
      <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">System (Admin)</h1>
      <p className="mt-1 text-[13px] text-white/35">
        System health and infrastructure overview
      </p>

      <div className="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Total Nodes" value="55" change="+8" icon={Server} />
        <StatCard title="Total Tenants" value="3" icon={Database} />
        <StatCard title="Active MICs" value="4,062" change="+320" icon={Shield} />
        <StatCard
          title="API Requests (24h)"
          value="142K"
          change="+18%"
          icon={Activity}
        />
      </div>

      {/* Services health */}
      <div className="mt-8 rounded-xl border border-white/[0.04] bg-white/[0.02] p-6">
        <h2 className="text-sm font-semibold text-white/70">
          Service Health
        </h2>
        <div className="mt-4 space-y-3">
          {[
            { name: "strand-cloud API", status: "healthy", latency: "2ms" },
            { name: "PostgreSQL", status: "healthy", latency: "1ms" },
            { name: "ClickHouse", status: "healthy", latency: "3ms" },
            { name: "Ory Kratos", status: "healthy", latency: "5ms" },
            { name: "etcd cluster", status: "healthy", latency: "1ms" },
          ].map((svc) => (
            <div
              key={svc.name}
              className="flex items-center justify-between text-sm"
            >
              <div className="flex items-center gap-3">
                <span className="h-2 w-2 rounded-full bg-green-400" />
                <span className="text-white/90">{svc.name}</span>
              </div>
              <div className="flex items-center gap-4">
                <span className="text-green-400">{svc.status}</span>
                <span className="font-mono text-white/25">{svc.latency}</span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Version info */}
      <div className="mt-8 rounded-xl border border-white/[0.04] bg-white/[0.02] p-6">
        <h2 className="text-sm font-semibold text-white/70">
          Version Info
        </h2>
        <div className="mt-4 grid gap-3 sm:grid-cols-2 text-sm">
          {[
            { label: "strand-cloud", value: "0.1.0" },
            { label: "PostgreSQL", value: "16.2" },
            { label: "ClickHouse", value: "24.2" },
            { label: "Ory Kratos", value: "1.1.0" },
            { label: "etcd", value: "3.5.12" },
            { label: "Go", value: "1.22.0" },
          ].map((item) => (
            <div
              key={item.label}
              className="flex items-center justify-between"
            >
              <span className="text-white/25">{item.label}</span>
              <span className="font-mono text-white/90">{item.value}</span>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
