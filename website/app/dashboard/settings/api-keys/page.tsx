import { ApiKeyRow } from "@/components/dashboard/api-key-row";

const demoKeys = [
  {
    name: "Production Deploy",
    prefix: "pk_live_",
    role: "admin",
    createdAt: "2025-11-15T00:00:00Z",
    lastUsed: "2026-02-25T12:30:00Z",
  },
  {
    name: "CI/CD Pipeline",
    prefix: "pk_live_",
    role: "operator",
    createdAt: "2026-01-10T00:00:00Z",
    lastUsed: "2026-02-26T08:00:00Z",
  },
  {
    name: "Monitoring Read-Only",
    prefix: "pk_live_",
    role: "viewer",
    createdAt: "2026-02-01T00:00:00Z",
    expiresAt: "2026-08-01T00:00:00Z",
  },
];

export default function ApiKeysPage() {
  return (
    <>
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">API Keys</h1>
          <p className="mt-1 text-[13px] text-white/35">
            Manage API keys for programmatic access
          </p>
        </div>
        <button className="rounded-full bg-white px-6 py-2.5 text-sm font-medium text-[#060609] transition-all hover:bg-white/90">
          Create Key
        </button>
      </div>

      <div className="mt-6 space-y-3">
        {demoKeys.map((key) => (
          <ApiKeyRow key={key.name} {...key} />
        ))}
      </div>
    </>
  );
}
