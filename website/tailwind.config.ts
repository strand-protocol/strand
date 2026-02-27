import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./lib/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        strand: {
          DEFAULT: "#6C5CE7",
          50: "#EDE9FC",
          100: "#D5CFF8",
          200: "#AEA1F1",
          300: "#8C77EB",
          400: "#6C5CE7",
          500: "#5A3ED4",
          600: "#4A2FBA",
          700: "#3A2494",
          800: "#2A1A6E",
          900: "#1A1048",
        },
        cyan: {
          DEFAULT: "#00D2FF",
          400: "#33DBFF",
          500: "#00D2FF",
          600: "#00A8CC",
        },
        coral: {
          DEFAULT: "#FF6B6B",
          400: "#FF8A8A",
          500: "#FF6B6B",
          600: "#FF4C4C",
        },
        bg: {
          DEFAULT: "#060609",
          surface: "#0c0c14",
          "surface-2": "#13131f",
        },
        border: {
          DEFAULT: "#1e1e2e",
          light: "#2a2a3e",
        },
        text: {
          primary: "#f0f0f5",
          secondary: "#94949e",
          muted: "#55555e",
        },
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "-apple-system", "sans-serif"],
        mono: ["JetBrains Mono", "Fira Code", "Consolas", "monospace"],
      },
      animation: {
        "fade-in": "fade-in 0.8s cubic-bezier(0.16, 1, 0.3, 1)",
        marquee: "marquee 30s linear infinite",
        "grid-drift": "grid-drift 25s linear infinite",
      },
      keyframes: {
        "fade-in": {
          "0%": { opacity: "0", transform: "translateY(12px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        marquee: {
          "0%": { transform: "translateX(0%)" },
          "100%": { transform: "translateX(-50%)" },
        },
        "grid-drift": {
          "0%": { transform: "translate(0, 0)" },
          "100%": { transform: "translate(80px, 80px)" },
        },
      },
    },
  },
  plugins: [require("@tailwindcss/typography")],
};

export default config;
