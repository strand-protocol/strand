import { NodeTable } from "@/components/dashboard/node-table";
import type { Node } from "@/lib/api";

// In production this would call api.nodes.list() server-side.
// For now, use static demo data so the page renders without a backend.
const demoNodes: Node[] = [
  {
    id: "node-gpu-a100-01",
    address: "10.0.1.10:6477",
    status: "online",
    last_seen: new Date().toISOString(),
    firmware_version: "0.1.0",
    metrics: { connections: 24, bytes_sent: 1073741824, bytes_recv: 2147483648, avg_latency: 1200000 },
  },
  {
    id: "node-gpu-a100-02",
    address: "10.0.1.11:6477",
    status: "online",
    last_seen: new Date(Date.now() - 30_000).toISOString(),
    firmware_version: "0.1.0",
    metrics: { connections: 18, bytes_sent: 536870912, bytes_recv: 1073741824, avg_latency: 800000 },
  },
  {
    id: "node-cpu-infer-01",
    address: "10.0.2.20:6477",
    status: "degraded",
    last_seen: new Date(Date.now() - 300_000).toISOString(),
    firmware_version: "0.0.9",
    metrics: { connections: 5, bytes_sent: 104857600, bytes_recv: 209715200, avg_latency: 5000000 },
  },
];

export default function NodesPage() {
  return (
    <>
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">Nodes</h1>
          <p className="mt-1 text-[13px] text-white/35">
            All registered Strand Protocol nodes in your clusters
          </p>
        </div>
      </div>
      <div className="mt-6">
        <NodeTable nodes={demoNodes} />
      </div>
    </>
  );
}
