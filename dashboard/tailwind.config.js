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
        // KubePilot brand palette — dark navy cockpit theme
        pilot: {
          bg:       "#0a0e1a",
          surface:  "#111827",
          border:   "#1f2937",
          accent:   "#3b82f6",
          success:  "#10b981",
          warning:  "#f59e0b",
          danger:   "#ef4444",
          muted:    "#6b7280",
        },
      },
      fontFamily: {
        mono: ["JetBrains Mono", "ui-monospace", "monospace"],
      },
    },
  },
  plugins: [],
};
