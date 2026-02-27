import { Sidebar } from "@/components/layout/sidebar";

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="relative mx-auto flex max-w-[1400px] gap-0 px-6 pt-28 pb-20 lg:gap-12">
      <Sidebar />
      <article className="min-w-0 flex-1 max-w-3xl">
        {children}
      </article>
    </div>
  );
}
