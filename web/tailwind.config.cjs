/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "../internal/features/**/*.templ",
    "../web/layout/**/*.templ",
    "../internal/features/**/*.go",
    "./src/**/*.js",
    "./static/js/**/*.js",
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
      },
      animation: {
        "fast-spin": "fast-spin 0.4s linear infinite",
      },
      keyframes: {
        "fast-spin": {
          from: { transform: "rotate(0deg)" },
          to: { transform: "rotate(360deg)" },
        },
      },
    },
  },
  plugins: [
    require('@tailwindcss/typography'),
  ],
}
