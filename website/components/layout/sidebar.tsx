"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { docsNav } from "@/lib/navigation";
import type { NavItem } from "@/lib/navigation";

function NavLink({ item }: { item: NavItem }) {
  const pathname = usePathname();
  const isActive = pathname === item.href;

  return (
    <Link
      href={item.href}
      className={`block rounded-lg px-3 py-1.5 text-[13px] transition-all duration-200 ${
        isActive
          ? "bg-white/[0.06] font-medium text-white/90"
          : "text-white/35 hover:bg-white/[0.03] hover:text-white/60"
      }`}
    >
      {item.title}
    </Link>
  );
}

export function Sidebar() {
  return (
    <aside className="hidden w-56 flex-shrink-0 lg:block">
      <nav className="sticky top-28 space-y-6">
        {docsNav.map((section) => (
          <div key={section.title}>
            <h4 className="mb-2 px-3 text-[11px] font-medium uppercase tracking-[0.12em] text-white/25">
              {section.title}
            </h4>
            <div className="space-y-0.5">
              {section.children?.map((item) => (
                <NavLink key={item.href} item={item} />
              ))}
            </div>
          </div>
        ))}
      </nav>
    </aside>
  );
}
