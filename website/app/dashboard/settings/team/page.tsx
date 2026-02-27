const demoMembers = [
  { email: "alice@acme.ai", name: "Alice Chen", role: "owner", lastActive: "1h ago" },
  { email: "bob@acme.ai", name: "Bob Martinez", role: "admin", lastActive: "3h ago" },
  { email: "carol@acme.ai", name: "Carol Wang", role: "operator", lastActive: "1d ago" },
];

const roleColors: Record<string, string> = {
  owner: "bg-coral/20 text-coral",
  admin: "bg-strand/20 text-strand-300",
  operator: "bg-cyan/20 text-cyan",
  viewer: "bg-white/[0.05] text-white/25",
};

export default function TeamPage() {
  return (
    <>
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-[-0.02em] text-white/90">Team</h1>
          <p className="mt-1 text-[13px] text-white/35">
            Manage team members and roles
          </p>
        </div>
        <button className="rounded-full bg-white px-6 py-2.5 text-sm font-medium text-[#060609] transition-all hover:bg-white/90">
          Invite Member
        </button>
      </div>

      <div className="mt-6 overflow-x-auto rounded-xl border border-white/[0.04]">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-white/[0.04] bg-white/[0.02] text-[11px] uppercase tracking-[0.15em] text-white/25">
            <tr>
              <th className="px-4 py-3 font-medium">Member</th>
              <th className="px-4 py-3 font-medium">Role</th>
              <th className="px-4 py-3 font-medium">Last Active</th>
              <th className="px-4 py-3 font-medium" />
            </tr>
          </thead>
          <tbody className="divide-y divide-white/[0.04]">
            {demoMembers.map((m) => (
              <tr
                key={m.email}
                className="transition-colors hover:bg-white/[0.02]"
              >
                <td className="px-4 py-3">
                  <div className="font-medium text-white/90">{m.name}</div>
                  <div className="text-white/25">{m.email}</div>
                </td>
                <td className="px-4 py-3">
                  <span
                    className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                      roleColors[m.role] || roleColors.viewer
                    }`}
                  >
                    {m.role}
                  </span>
                </td>
                <td className="px-4 py-3 text-white/25">{m.lastActive}</td>
                <td className="px-4 py-3 text-right">
                  <button className="text-sm text-white/25 hover:text-white/90">
                    Edit
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
