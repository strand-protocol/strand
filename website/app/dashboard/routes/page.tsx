const demoRoutes = [
  {
    id: "inference-pool-us-west",
    endpoints: 3,
    capabilities: ["text-generation", "code-completion"],
    ttl: "30s",
  },
  {
    id: "embedding-service",
    endpoints: 2,
    capabilities: ["embeddings"],
    ttl: "60s",
  },
  {
    id: "vision-model-pool",
    endpoints: 1,
    capabilities: ["image-understanding", "ocr"],
    ttl: "30s",
  },
];

export default function RoutesPage() {
  return (
    <>
      <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">Routes</h1>
      <p className="mt-1 text-[13px] text-white/35">
        Semantic Address Descriptor (SAD) routes
      </p>

      <div className="mt-6 overflow-x-auto rounded-xl border border-white/[0.04]">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-white/[0.04] bg-white/[0.02] text-[11px] uppercase tracking-[0.15em] text-white/25">
            <tr>
              <th className="px-4 py-3 font-medium">Route ID</th>
              <th className="px-4 py-3 font-medium">Endpoints</th>
              <th className="px-4 py-3 font-medium">Capabilities</th>
              <th className="px-4 py-3 font-medium">TTL</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-white/[0.04]">
            {demoRoutes.map((route) => (
              <tr
                key={route.id}
                className="transition-colors hover:bg-white/[0.02]"
              >
                <td className="px-4 py-3 font-mono text-strand">
                  {route.id}
                </td>
                <td className="px-4 py-3 text-white/40">
                  {route.endpoints}
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-1">
                    {route.capabilities.map((cap) => (
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
                  {route.ttl}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
