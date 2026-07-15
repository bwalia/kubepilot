/** @type {import('tailwindcss').Config} */

// Every "pilot" colour is a CSS custom property holding space-separated RGB
// channels, wrapped in rgb(... / <alpha-value>) so Tailwind opacity utilities
// (e.g. bg-pilot-accent/12) keep working. The channel values live in
// styles/globals.css and are redefined per theme — flip :root[data-theme] and
// the entire UI re-skins between "Daylight" (light) and "Night" (dark).
const tone = (name) => `rgb(var(--p-${name}) / <alpha-value>)`;

module.exports = {
  darkMode: ["class", '[data-theme="dark"]'],
  content: [
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        // KubePilot palette — theme-driven. Daylight = high-contrast light deck,
        // Night = the classic cyan-phosphor Flight Deck. Status colours stay
        // universal and unambiguous in both themes.
        pilot: {
          bg:               tone("bg"),
          surface:          tone("surface"),
          "surface-2":      tone("surface-2"),
          border:           tone("border"),
          "border-hover":   tone("border-hover"),
          accent:           tone("accent"),
          "accent-light":   tone("accent-light"),
          "accent-deep":    tone("accent-deep"),
          success:          tone("success"),
          warning:          tone("warning"),
          danger:           tone("danger"),
          info:             tone("info"),
          muted:            tone("muted"),
          "text-secondary": tone("text-secondary"),
          "text-primary":   tone("text-primary"),
          // Neutral hover tint: near-black in Daylight, near-white in Night —
          // so hover overlays read correctly on either ground.
          hover:            tone("hover"),
        },
      },
      fontFamily: {
        // IBM Plex — engineered for data-dense, all-day interfaces. Sans for
        // UI/prose, Mono for identifiers & figures. Cohesive, unambiguous glyphs.
        sans: ["'IBM Plex Sans'", "-apple-system", "BlinkMacSystemFont", "Segoe UI", "Roboto", "Helvetica Neue", "Arial", "sans-serif"],
        display: ["'IBM Plex Sans'", "-apple-system", "sans-serif"],
        mono: ["'IBM Plex Mono'", "ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
      fontSize: {
        // Readability pass: the whole small-text ramp is nudged up so nothing
        // reads as tiny. Combined with the 17px root (globals.css) this lifts
        // every text-2xs / text-xs / text-sm across the app in one place.
        "2xs": ["0.8rem", { lineHeight: "1.1rem" }],   // was 11px → ~13.6px
        xs: ["0.85rem", { lineHeight: "1.2rem" }],      // was 12px → ~14.5px
        sm: ["0.925rem", { lineHeight: "1.35rem" }],    // was 14px → ~15.7px
      },
      letterSpacing: {
        eyebrow: "0.1em",
      },
      borderRadius: {
        xl: "12px",
        "2xl": "16px",
      },
      boxShadow: {
        // Theme-aware elevation. Daylight uses soft ambient shadows; Night keeps
        // the instrument-bezel highlight-over-depth. Defined in globals.css.
        card: "var(--shadow-card)",
        "card-hover": "var(--shadow-card-hover)",
        bar: "var(--shadow-bar)",
        "glow-blue": "var(--glow-blue)",
        "glow-red": "var(--glow-red)",
        inset: "var(--shadow-inset)",
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
