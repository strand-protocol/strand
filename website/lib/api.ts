const API_BASE = process.env.STRAND_API_URL || "http://localhost:8080";

interface FetchOptions {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
}

export async function apiFetch<T>(
  path: string,
  opts: FetchOptions = {}
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: opts.method || "GET",
    headers: {
      "Content-Type": "application/json",
      ...opts.headers,
    },
    body: opts.body ? JSON.stringify(opts.body) : undefined,
    cache: "no-store",
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `API error ${res.status}`);
  }
  return res.json();
}

// Typed API helpers
export const api = {
  nodes: {
    list: () => apiFetch<Node[]>("/api/v1/nodes"),
    get: (id: string) => apiFetch<Node>(`/api/v1/nodes/${id}`),
  },
  clusters: {
    list: (tenantId?: string) =>
      apiFetch<Cluster[]>(
        `/api/v1/clusters${tenantId ? `?tenant_id=${tenantId}` : ""}`
      ),
    get: (id: string) => apiFetch<Cluster>(`/api/v1/clusters/${id}`),
  },
  tenants: {
    list: () => apiFetch<Tenant[]>("/api/v1/tenants"),
    get: (id: string) => apiFetch<Tenant>(`/api/v1/tenants/${id}`),
  },
  routes: {
    list: () => apiFetch<Route[]>("/api/v1/routes"),
  },
  mics: {
    list: () => apiFetch<MIC[]>("/api/v1/trust/mics"),
  },
  firmware: {
    list: () => apiFetch<FirmwareImage[]>("/api/v1/firmware"),
  },
  audit: {
    list: (tenantId?: string, limit?: number) =>
      apiFetch<AuditEntry[]>(
        `/api/v1/audit?${tenantId ? `tenant_id=${tenantId}&` : ""}limit=${limit || 100}`
      ),
  },
};

// Types matching strand-cloud models
export interface Node {
  id: string;
  address: string;
  status: string;
  last_seen: string;
  firmware_version: string;
  metrics: {
    connections: number;
    bytes_sent: number;
    bytes_recv: number;
    avg_latency: number;
  };
}

export interface Cluster {
  id: string;
  tenant_id: string;
  name: string;
  region: string;
  status: string;
  control_plane_endpoint: string;
  node_count: number;
  created_at: string;
  updated_at: string;
}

export interface Tenant {
  id: string;
  name: string;
  slug: string;
  plan: string;
  status: string;
  max_clusters: number;
  max_nodes: number;
  max_mics_month: number;
  traffic_gb_included: number;
  created_at: string;
  updated_at: string;
}

export interface Route {
  id: string;
  sad: string;
  endpoints: { node_id: string; address: string; weight: number }[];
  ttl: number;
  created_at: string;
}

export interface MIC {
  id: string;
  node_id: string;
  capabilities: string[];
  valid_from: string;
  valid_until: string;
  revoked: boolean;
}

export interface FirmwareImage {
  id: string;
  version: string;
  platform: string;
  size: number;
  checksum: string;
  url: string;
  created_at: string;
}

export interface AuditEntry {
  id: string;
  tenant_id: string;
  actor_id: string;
  actor_type: string;
  action: string;
  resource_type: string;
  resource_id: string;
  created_at: string;
}
