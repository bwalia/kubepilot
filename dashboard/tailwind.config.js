/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        // KubePilot "Flight Deck" palette — an avionics instrument panel.
        // Deep teal-ink neutrals + a cyan-phosphor primary. Status colours
        // (green / amber / red) stay universal and unambiguous.
        pilot: {
          bg:            "#081214",
          surface:       "#0f1e21",
          "surface-2":   "#16292d",
          border:        "#1e343a",
          "border-hover": "#2d4f57",
          // cyan phosphor primary
          accent:        "#22d3ee",
          "accent-light": "#67e8f9",
          "accent-deep": "#0e7490",
          // status
          success:       "#22c55e",
          warning:       "#f5a623",
          danger:        "#f5555d",
          info:          "#a78bfa",
          // text ramp (raised contrast vs. the old muted grey)
          muted:         "#8ba3aa",
          "text-secondary": "#c2d4d8",
          "text-primary": "#e8f1f2",
        },
      },
      fontFamily: {
        // Data & prose: Inter (unbeatable at small tabular sizes)
        sans: ["Inter", "-apple-system", "BlinkMacSystemFont", "Segoe UI", "Roboto", "Helvetica Neue", "Arial", "sans-serif"],
        // Headings & instrument labels: Space Grotesk (engineered character)
        display: ["Space Grotesk", "Inter", "-apple-system", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "monospace"],
      },
      fontSize: {
        "2xs": ["0.6875rem", { lineHeight: "1rem" }],
      },
      letterSpacing: {
        eyebrow: "0.12em",
      },
      borderRadius: {
        xl: "12px",
        "2xl": "16px",
      },
      boxShadow: {
        // Instrument-bezel elevation: a hairline top highlight over depth.
        card: "inset 0 1px 0 0 rgba(255,255,255,0.05), 0 1px 2px rgba(0,0,0,0.45), 0 10px 28px -16px rgba(0,0,0,0.65)",
        "card-hover": "inset 0 1px 0 0 rgba(255,255,255,0.08), 0 2px 6px rgba(0,0,0,0.5), 0 16px 40px -14px rgba(0,0,0,0.75)",
        "glow-blue": "0 0 0 1px rgba(34,211,238,0.30), 0 0 26px -6px rgba(34,211,238,0.45)",
        "glow-red": "0 0 0 1px rgba(245,85,93,0.30), 0 0 26px -6px rgba(245,85,93,0.45)",
        inset: "inset 0 1px 0 0 rgba(255,255,255,0.05)",
      },
      animation: {
        "fade-in": "fadeIn 0.2s ease-out",
        "slide-in-right": "slideInRight 0.25s ease-out",
        "pulse-dot": "pulseDot 2s ease-in-out infinite",
      },
      keyframes: {
        fadeIn: {
          "0%": { opacity: "0", transform: "translateY(4px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        slideInRight: {
          "0%": { opacity: "0", transform: "translateX(16px)" },
          "100%": { opacity: "1", transform: "translateX(0)" },
        },
        pulseDot: {
          "0%, 100%": { opacity: "1", transform: "scale(1)" },
          "50%": { opacity: "0.4", transform: "scale(0.82)" },
        },
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
};
