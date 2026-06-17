import type { Config } from "tailwindcss";

/**
 * ElaMachan design tokens (VER-240 / approved on VER-239).
 * Light, premium marketplace. Yellow = brand / primary action,
 * Blue = trust / links / secondary action. No orange.
 * Source of truth: design/ver-239/elamachan-redesign.html
 */
const config: Config = {
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        // Map to CSS variables so the palette stays single-sourced in globals.css
        background: "var(--c-bg)",
        surface: "var(--c-surface)",
        "surface-2": "var(--c-surface-2)",
        foreground: "var(--c-ink)",
        ink: {
          DEFAULT: "var(--c-ink)",
          2: "var(--c-ink-2)",
        },
        muted: "var(--c-muted)",
        border: "var(--c-border)",
        dark: "var(--c-dark)",
        brand: {
          DEFAULT: "#F5B301", // primary yellow
          600: "#D99700", // hover / pressed
          soft: "#FFF3CD", // tinted surface
        },
        accent: {
          DEFAULT: "#2563EB", // secondary blue
          700: "#1D4ED8", // hover / pressed
          soft: "#EAF1FE", // tinted surface
        },
        success: "#15803D", // price / positive
      },
      fontSize: {
        display: ["var(--fs-display)", { lineHeight: "1", letterSpacing: "-0.025em" }],
        h2: ["var(--fs-h2)", { lineHeight: "1.2", letterSpacing: "-0.02em" }],
        h3: ["var(--fs-h3)", { lineHeight: "1.2" }],
        body: ["var(--fs-body)", { lineHeight: "1.5" }],
        small: ["var(--fs-sm)", { lineHeight: "1.4" }],
        caption: ["var(--fs-xs)", { lineHeight: "1.3" }],
      },
      borderRadius: {
        sm: "8px",
        md: "12px",
        lg: "18px",
        pill: "999px",
      },
      boxShadow: {
        sm: "0 1px 2px rgba(16,16,29,.06), 0 1px 1px rgba(16,16,29,.04)",
        md: "0 4px 14px rgba(16,16,29,.08)",
        lg: "0 18px 40px rgba(16,16,29,.14)",
      },
      maxWidth: {
        wrap: "1280px",
      },
    },
  },
  plugins: [],
};

export default config;
