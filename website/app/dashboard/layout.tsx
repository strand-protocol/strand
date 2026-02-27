import { DashboardSidebar } from "@/components/dashboard/sidebar";
import { DashboardHeader } from "@/components/dashboard/header";

export const metadata = {
  title: "Dashboard",
};

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen bg-[#060609]">
      <DashboardHeader />
      <DashboardSidebar />
      <div className="lg:pl-56">
        <div className="mx-auto max-w-[1200px] px-6 py-8">{children}</div>
      </div>
    </div>
  );
}
