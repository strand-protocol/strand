"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  Server,
  GitBranch,
  Route,
  Shield,
  Activity,
  CreditCard,
  Settings,
  Key,
  Users,
  Building,
  Gauge,
} from "lucide-react";

const navItems = [
  { href: "/dashboard", label: "Overview", icon: LayoutDashboard },
  { href: "/dashboard/nodes", label: "Nodes", icon: Server },
  { href: "/dashboard/clusters", label: "Clusters", icon: GitBranch },
  { href: "/dashboard/routes", label: "Routes", icon: Route },
  { href: "/dashboard/trust", label: "Trust / MICs", icon: Shield },
  { href: "/dashboard/traffic", label: "Traffic", icon: Activity },
  { href: "/dashboard/billing", label: "Billing", icon: CreditCard },
];

const settingsItems = [
  { href: "/dashboard/settings", label: "General", icon: Settings },
  { href: "/dashboard/settings/api-keys", label: "API Keys", icon: Key },
  { href: "/dashboard/settings/team", label: "Team", icon: Users },
];

const adminItems = [
  { href: "/dashboard/admin/tenants", label: "Tenants", icon: Building },
  { href: "/dashboard/admin/system", label: "System", icon: Gauge },
];

function NavSection({
  title,
  items,
}: {
  title: string;
  items: typeof navItems;
}) {
  const pathname = usePathname();
  return (
    <div className="mb-8">
      <h3 className="mb-3 px-3 text-[11px] font-medium uppercase tracking-[0.15em] text-white/20">
        {title}
      </h3>
      <ul className="space-y-0.5">
        {items.map((item) => {
          const active = pathname === item.href;
          return (
            <li key={item.href}>
              <Link
                href={item.href}
                className={`flex items-center gap-3 rounded-lg px-3 py-2 text-[13px] transition-all duration-200 ${
                  active
                    ? "bg-white/[0.06] text-white/90 font-medium"
                    : "text-white/35 hover:bg-white/[0.03] hover:text-white/60"
                }`}
              >
                <item.icon className="h-4 w-4" />
                {item.label}
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export function DashboardSidebar() {
  return (
    <aside className="fixed left-0 top-14 z-30 hidden h-[calc(100vh-3.5rem)] w-56 border-r border-white/[0.04] bg-[#060609] lg:block">
      <nav className="h-full overflow-y-auto p-4 pt-6">
        <NavSection title="Platform" items={navItems} />
        <NavSection title="Settings" items={settingsItems} />
        <NavSection title="Admin" items={adminItems} />
      </nav>
    </aside>
  );
}
