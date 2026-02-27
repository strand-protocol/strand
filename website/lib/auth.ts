import { cookies } from "next/headers";

const KRATOS_URL = process.env.STRAND_KRATOS_URL || "http://localhost:4433";

export interface SessionUser {
  id: string;
  email: string;
  name: string;
  tenantId: string;
  role: "viewer" | "operator" | "admin" | "owner";
}

export async function getSession(): Promise<SessionUser | null> {
  const cookieStore = await cookies();
  const sessionCookie = cookieStore.get("ory_kratos_session");
  if (!sessionCookie) return null;

  try {
    const res = await fetch(`${KRATOS_URL}/sessions/whoami`, {
      headers: { Cookie: `ory_kratos_session=${sessionCookie.value}` },
      cache: "no-store",
    });
    if (!res.ok) return null;
    const session = await res.json();
    return {
      id: session.identity?.id || "",
      email: session.identity?.traits?.email || "",
      name: session.identity?.traits?.name || "",
      tenantId: session.identity?.metadata_public?.tenant_id || "",
      role: session.identity?.metadata_public?.role || "viewer",
    };
  } catch {
    return null;
  }
}
