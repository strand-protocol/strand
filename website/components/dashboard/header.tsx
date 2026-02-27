"use client";

import Link from "next/link";
import { Bell, Search } from "lucide-react";

export function DashboardHeader() {
  return (
    <header className="sticky top-0 z-40 flex h-14 items-center justify-between border-b border-white/[0.04] bg-[#060609]/80 px-6 backdrop-blur-xl">
      <div className="flex items-center gap-3">
        <Link href="/dashboard" className="flex items-center gap-2.5">
          <svg
            viewBox="0 0 32 32"
            className="h-5 w-5"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
          >
            <path
              d="M4 16h5l3-10 4 20 4-20 3 10h5"
              stroke="#6C5CE7"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          <span className="text-sm font-medium tracking-[-0.02em] text-white/80">
            Strand Cloud
          </span>
        </Link>
      </div>

      <div className="flex items-center gap-3">
        {/* Search */}
        <div className="hidden items-center gap-2 rounded-lg border border-white/[0.06] bg-white/[0.02] px-3 py-1.5 text-[13px] text-white/20 sm:flex">
          <Search className="h-3.5 w-3.5" />
          <span>Search...</span>
          <kbd className="rounded border border-white/[0.06] px-1.5 py-0.5 text-[11px] text-white/15">
            /
          </kbd>
        </div>

        <button className="relative rounded-lg p-2 text-white/25 transition-colors duration-200 hover:bg-white/[0.04] hover:text-white/50">
          <Bell className="h-4 w-4" />
          <span className="absolute right-1.5 top-1.5 h-1.5 w-1.5 rounded-full bg-strand" />
        </button>

        <div className="h-7 w-7 rounded-full bg-strand/15 text-center text-xs font-medium leading-7 text-strand-300">
          P
        </div>
      </div>
    </header>
  );
}
