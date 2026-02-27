"use client";

import { Eye, EyeOff, Trash2 } from "lucide-react";
import { useState } from "react";
import { formatDate } from "@/lib/format";

interface ApiKeyRowProps {
  name: string;
  prefix: string;
  role: string;
  createdAt: string;
  lastUsed?: string;
  expiresAt?: string;
}

export function ApiKeyRow({
  name,
  prefix,
  role,
  createdAt,
  lastUsed,
  expiresAt,
}: ApiKeyRowProps) {
  const [revealed, setRevealed] = useState(false);

  return (
    <div className="flex items-center justify-between rounded-xl border border-white/[0.04] bg-white/[0.02] px-4 py-3 transition-colors duration-200 hover:border-white/[0.08]">
      <div className="flex flex-col gap-1">
        <div className="flex items-center gap-3">
          <span className="text-sm font-medium text-white/80">{name}</span>
          <span className="rounded-md border border-white/[0.06] bg-white/[0.02] px-2 py-0.5 text-[11px] text-white/30">
            {role}
          </span>
        </div>
        <div className="flex items-center gap-2 font-mono text-[13px] text-white/25">
          <span>{prefix}{"••••••••••••••••"}</span>
          <button
            onClick={() => setRevealed(!revealed)}
            className="text-white/20 hover:text-white/40"
          >
            {revealed ? (
              <EyeOff className="h-3.5 w-3.5" />
            ) : (
              <Eye className="h-3.5 w-3.5" />
            )}
          </button>
        </div>
      </div>

      <div className="flex items-center gap-6 text-[13px] text-white/25">
        <div className="hidden sm:block">
          <div className="text-[11px] text-white/15">Created</div>
          <div>{formatDate(createdAt)}</div>
        </div>
        {lastUsed && (
          <div className="hidden md:block">
            <div className="text-[11px] text-white/15">Last used</div>
            <div>{formatDate(lastUsed)}</div>
          </div>
        )}
        {expiresAt && (
          <div className="hidden md:block">
            <div className="text-[11px] text-white/15">Expires</div>
            <div>{formatDate(expiresAt)}</div>
          </div>
        )}
        <button className="rounded-lg p-2 text-white/20 transition-colors duration-200 hover:bg-red-500/10 hover:text-red-400/70">
          <Trash2 className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
