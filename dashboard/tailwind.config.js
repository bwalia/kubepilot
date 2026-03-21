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
        // KubePilot brand palette — refined dark cockpit theme
        pilot: {
          bg:       "#0b1120",
          surface:  "#131c2e",
          "surface-2": "#1a2540",
          border:   "#1e2d4a",
          "border-hover": "#2a3f66",
          accent:   "#3b82f6",
          "accent-light": "#60a5fa",
          success:  "#10b981",
          warning:  "#f59e0b",
          danger:   "#ef4444",
          muted:    "#7c8ba3",
          "text-secondary": "#b0bdd0",
        },
      },
      fontFamily: {
        sans: ["Inter", "-apple-system", "BlinkMacSystemFont", "Segoe UI", "Roboto", "Helvetica Neue", "Arial", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "monospace"],
      },
      fontSize: {
        "2xs": ["0.6875rem", { lineHeight: "1rem" }],
      },
      borderRadius: {
        "xl": "12px",
        "2xl": "16px",
      },
      boxShadow: {
        "glow-blue": "0 0 20px rgba(59, 130, 246, 0.15)",
        "glow-red": "0 0 20px rgba(239, 68, 68, 0.15)",
        "card": "0 1px 3px rgba(0,0,0,0.3), 0 1px 2px rgba(0,0,0,0.2)",
        "card-hover": "0 4px 12px rgba(0,0,0,0.4), 0 2px 4px rgba(0,0,0,0.3)",
      },
      animation: {
        "fade-in": "fadeIn 0.2s ease-out",
        "slide-in-right": "slideInRight 0.25s ease-out",
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
      },
    },
  },
  plugins: [],
};
