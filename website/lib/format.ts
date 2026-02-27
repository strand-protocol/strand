export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

export function formatNumber(n: number): string {
  return new Intl.NumberFormat().format(n);
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export function formatRelative(iso: string): string {
  const now = Date.now();
  const then = new Date(iso).getTime();
  const diff = now - then;
  if (diff < 60_000) return "just now";
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
  return `${Math.floor(diff / 86_400_000)}d ago`;
}

export function statusColor(status: string): string {
  switch (status) {
    case "online":
    case "active":
    case "running":
      return "text-emerald-400/70";
    case "degraded":
    case "provisioning":
      return "text-amber-400/70";
    case "offline":
    case "suspended":
    case "cancelled":
      return "text-red-400/70";
    default:
      return "text-white/25";
  }
}

export function planBadgeColor(plan: string): string {
  switch (plan) {
    case "free":
      return "bg-white/[0.06] text-white/40";
    case "starter":
      return "bg-cyan/10 text-cyan/70";
    case "pro":
      return "bg-strand/10 text-strand-300";
    case "enterprise":
      return "bg-coral/10 text-coral/70";
    default:
      return "bg-white/[0.06] text-white/40";
  }
}
