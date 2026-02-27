import { statusColor } from "@/lib/format";

const demoMICs = [
  {
    id: "mic-a3f8b2c1",
    nodeId: "node-gpu-a100-01",
    capabilities: ["text-generation", "code-completion"],
    validUntil: "2026-06-01T00:00:00Z",
    revoked: false,
  },
  {
    id: "mic-d4e9c3a2",
    nodeId: "node-gpu-a100-02",
    capabilities: ["text-generation"],
    validUntil: "2026-06-01T00:00:00Z",
    revoked: false,
  },
  {
    id: "mic-b1c4d5e6",
    nodeId: "node-cpu-infer-01",
    capabilities: ["embeddings"],
    validUntil: "2026-03-15T00:00:00Z",
    revoked: true,
  },
];

export default function TrustPage() {
  return (
    <>
      <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">Trust / MICs</h1>
      <p className="mt-1 text-[13px] text-white/35">
        Model Identity Certificates issued by StrandTrust CA
      </p>

      <div className="mt-6 overflow-x-auto rounded-xl border border-white/[0.04]">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-white/[0.04] bg-white/[0.02] text-[11px] uppercase tracking-[0.15em] text-white/25">
            <tr>
              <th className="px-4 py-3 font-medium">MIC ID</th>
              <th className="px-4 py-3 font-medium">Node</th>
              <th className="px-4 py-3 font-medium">Capabilities</th>
              <th className="px-4 py-3 font-medium">Valid Until</th>
              <th className="px-4 py-3 font-medium">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-white/[0.04]">
            {demoMICs.map((mic) => (
              <tr
                key={mic.id}
                className="transition-colors hover:bg-white/[0.02]"
              >
                <td className="px-4 py-3 font-mono text-strand">{mic.id}</td>
                <td className="px-4 py-3 font-mono text-white/40">
                  {mic.nodeId}
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-1">
                    {mic.capabilities.map((cap) => (
                      <span
                        key={cap}
                        className="rounded-md bg-white/[0.03] px-2 py-0.5 text-xs text-white/25"
                      >
                        {cap}
                      </span>
                    ))}
                  </div>
                </td>
                <td className="px-4 py-3 font-mono text-white/25">
                  {new Date(mic.validUntil).toLocaleDateString()}
                </td>
                <td className="px-4 py-3">
                  <span
                    className={`font-medium ${
                      mic.revoked
                        ? statusColor("offline")
                        : statusColor("online")
                    }`}
                  >
                    {mic.revoked ? "Revoked" : "Active"}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
