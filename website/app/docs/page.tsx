import Link from "next/link";
import { brand } from "@/lib/brand";
import {
  ArrowRight,
  Layers,
  Zap,
  Terminal,
} from "lucide-react";

const quickStart = [
  {
    title: "Quick Start",
    description:
      "Get Strand Protocol running in under 5 minutes with the pure-Go overlay transport. Zero CGo dependencies required.",
    href: "/docs/getting-started",
    icon: Zap,
  },
  {
    title: "Architecture",
    description:
      "Understand the five-layer protocol stack, dependency graph, two transport paths, and wire format.",
    href: "/docs/architecture",
    icon: Layers,
  },
  {
    title: "CLI Reference",
    description:
      "Install StrandCtl and manage nodes, routes, MICs, streams, and firmware from the command line.",
    href: "/docs/modules/strandctl",
    icon: Terminal,
  },
];

export default function DocsPage() {
  return (
    <>
      {/* Hero */}
      <div className="mb-16">
        <p className="text-[11px] font-medium uppercase tracking-[0.2em] text-strand-300">
          Documentation
        </p>
        <h1 className="mt-3 text-3xl font-bold tracking-[-0.03em] text-white/90 sm:text-4xl">
          Strand Protocol
        </h1>
        <p className="mt-4 max-w-2xl text-[15px] leading-[1.8] text-white/40">
          A ground-up replacement for the traditional TCP/IP network stack,
          purpose-built for AI inference, model identity, semantic routing, and
          agent-to-agent communication.
        </p>
      </div>

      {/* Quick start cards */}
      <div className="grid gap-3 sm:grid-cols-3">
        {quickStart.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className="group rounded-xl border border-white/[0.04] bg-white/[0.02] p-6 transition-all duration-300 hover:border-white/[0.08] hover:bg-white/[0.04]"
          >
            <item.icon className="h-5 w-5 text-white/25 transition-colors group-hover:text-strand-300" />
            <h3 className="mt-4 text-sm font-semibold text-white/80 group-hover:text-white/90">
              {item.title}
            </h3>
            <p className="mt-2 text-[13px] leading-relaxed text-white/30">
              {item.description}
            </p>
            <span className="mt-4 inline-flex items-center gap-1.5 text-[12px] font-medium text-strand-300/60 transition-colors group-hover:text-strand-300">
              Read more
              <ArrowRight className="h-3 w-3" />
            </span>
          </Link>
        ))}
      </div>

      {/* Protocol stack table */}
      <div className="mt-16">
        <h2 className="mb-1 text-lg font-bold tracking-[-0.02em] text-white/90">
          Protocol Stack
        </h2>
        <p className="mb-6 text-[13px] text-white/30">
          Seven modules organized into five protocol layers plus a control
          plane.
        </p>

        <div className="overflow-x-auto rounded-xl border border-white/[0.04]">
          <table className="w-full text-left text-[13px]">
            <thead className="border-b border-white/[0.04] bg-white/[0.02]">
              <tr>
                <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-[0.12em] text-white/25">
                  Layer
                </th>
                <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-[0.12em] text-white/25">
                  Module
                </th>
                <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-[0.12em] text-white/25">
                  Language
                </th>
                <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-[0.12em] text-white/25">
                  Role
                </th>
              </tr>
            </thead>
            <tbody>
              {brand.modules.map((m) => (
                <tr
                  key={m.name}
                  className="border-b border-white/[0.03] last:border-0 transition-colors hover:bg-white/[0.015]"
                >
                  <td className="px-4 py-2.5">
                    <span
                      className="inline-flex h-6 w-6 items-center justify-center rounded text-[10px] font-bold"
                      style={{
                        backgroundColor: `${m.color}15`,
                        color: m.color,
                      }}
                    >
                      {m.layer}
                    </span>
                  </td>
                  <td className="px-4 py-2.5">
                    <Link
                      href={`/docs/modules/${m.name.toLowerCase().replace(" ", "-")}`}
                      className="font-medium text-white/70 hover:text-strand-300 transition-colors"
                    >
                      {m.name}
                    </Link>
                  </td>
                  <td className="px-4 py-2.5">
                    <span className="rounded-md border border-white/[0.06] bg-white/[0.02] px-2 py-0.5 font-mono text-[11px] text-white/30">
                      {m.lang}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-white/40">{m.role}</td>
                </tr>
              ))}
              {brand.controlPlane.map((cp) => (
                <tr
                  key={cp.name}
                  className="border-b border-white/[0.03] last:border-0 transition-colors hover:bg-white/[0.015]"
                >
                  <td className="px-4 py-2.5">
                    <span className="inline-flex h-6 w-6 items-center justify-center rounded bg-strand/8 text-[10px] font-bold text-strand-300">
                      CP
                    </span>
                  </td>
                  <td className="px-4 py-2.5">
                    <Link
                      href={`/docs/modules/${cp.name.toLowerCase().replace(" ", "-")}`}
                      className="font-medium text-white/70 hover:text-strand-300 transition-colors"
                    >
                      {cp.name}
                    </Link>
                  </td>
                  <td className="px-4 py-2.5">
                    <span className="rounded-md border border-white/[0.06] bg-white/[0.02] px-2 py-0.5 font-mono text-[11px] text-white/30">
                      {cp.lang}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-white/40">{cp.role}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Why section */}
      <div className="mt-16">
        <h2 className="mb-1 text-lg font-bold tracking-[-0.02em] text-white/90">
          Why Strand Protocol?
        </h2>
        <p className="max-w-2xl text-[15px] leading-[1.8] text-white/40">
          Today&apos;s AI infrastructure runs on networking primitives designed
          in the 1970s. HTTP for inference APIs, TCP for reliability, IP routing
          for addressing, TLS for identity. Strand Protocol replaces all of these
          with AI-native alternatives: semantic routing by model capabilities,
          cryptographic model identity, four delivery modes on one connection,
          and 18 purpose-built message types for inference, streaming, tensors,
          and agent communication.
        </p>
      </div>
    </>
  );
}
