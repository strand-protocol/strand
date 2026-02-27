export interface NavItem {
  title: string;
  href: string;
  children?: NavItem[];
}

export const mainNav: NavItem[] = [
  { title: "Features", href: "/#features" },
  { title: "Pricing", href: "/pricing" },
  { title: "Docs", href: "/docs" },
  { title: "Blog", href: "/blog" },
  { title: "Changelog", href: "/changelog" },
];

export const docsNav: NavItem[] = [
  {
    title: "Overview",
    href: "/docs",
    children: [
      { title: "Introduction", href: "/docs" },
      { title: "Getting Started", href: "/docs/getting-started" },
      { title: "Architecture", href: "/docs/architecture" },
    ],
  },
  {
    title: "Modules",
    href: "/docs/modules/strandlink",
    children: [
      { title: "StrandLink (L1)", href: "/docs/modules/strandlink" },
      { title: "StrandRoute (L2)", href: "/docs/modules/strandroute" },
      { title: "StrandStream (L3)", href: "/docs/modules/strandstream" },
      { title: "StrandTrust (L4)", href: "/docs/modules/strandtrust" },
      { title: "StrandAPI (L5)", href: "/docs/modules/strandapi" },
      { title: "StrandCtl", href: "/docs/modules/strandctl" },
      { title: "Strand Cloud", href: "/docs/modules/strand-cloud" },
    ],
  },
];

export const footerNav = {
  protocol: [
    { title: "Overview", href: "/docs" },
    { title: "Architecture", href: "/docs/architecture" },
    { title: "Getting Started", href: "/docs/getting-started" },
  ],
  modules: [
    { title: "StrandLink", href: "/docs/modules/strandlink" },
    { title: "StrandRoute", href: "/docs/modules/strandroute" },
    { title: "StrandAPI", href: "/docs/modules/strandapi" },
  ],
  community: [
    { title: "GitHub", href: "https://github.com/strand-protocol/strand" },
    { title: "Contributing", href: "https://github.com/strand-protocol/strand/blob/main/CONTRIBUTING.md" },
    { title: "License", href: "https://github.com/strand-protocol/strand/blob/main/LICENSE" },
  ],
};
