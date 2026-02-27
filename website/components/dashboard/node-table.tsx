import type { Node } from "@/lib/api";
import { formatRelative, formatBytes, statusColor } from "@/lib/format";
import Link from "next/link";

export function NodeTable({ nodes }: { nodes: Node[] }) {
  return (
    <div className="overflow-x-auto rounded-xl border border-white/[0.04]">
      <table className="w-full text-left text-[13px]">
        <thead className="border-b border-white/[0.04] bg-white/[0.02]">
          <tr className="text-[11px] uppercase tracking-[0.15em] text-white/25">
            <th className="px-4 py-3 font-medium">Node ID</th>
            <th className="px-4 py-3 font-medium">Status</th>
            <th className="px-4 py-3 font-medium">Address</th>
            <th className="px-4 py-3 font-medium">Connections</th>
            <th className="px-4 py-3 font-medium">Traffic</th>
            <th className="px-4 py-3 font-medium">Last Seen</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-white/[0.03]">
          {nodes.map((node) => (
            <tr
              key={node.id}
              className="transition-colors duration-200 hover:bg-white/[0.02]"
            >
              <td className="px-4 py-3">
                <Link
                  href={`/dashboard/nodes/${node.id}`}
                  className="font-mono text-strand-300 hover:text-strand-200"
                >
                  {node.id.slice(0, 12)}...
                </Link>
              </td>
              <td className="px-4 py-3">
                <span className={`font-medium ${statusColor(node.status)}`}>
                  {node.status}
                </span>
              </td>
              <td className="px-4 py-3 font-mono text-white/40">
                {node.address}
              </td>
              <td className="px-4 py-3 text-white/40">
                {node.metrics.connections}
              </td>
              <td className="px-4 py-3 text-white/40">
                {formatBytes(node.metrics.bytes_sent + node.metrics.bytes_recv)}
              </td>
              <td className="px-4 py-3 text-white/25">
                {formatRelative(node.last_seen)}
              </td>
            </tr>
          ))}
          {nodes.length === 0 && (
            <tr>
              <td
                colSpan={6}
                className="px-4 py-12 text-center text-white/20"
              >
                No nodes registered yet
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
