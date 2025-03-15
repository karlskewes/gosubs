/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [ "./**/*.templ", ],
  theme: {
    extend: {},
  },
  variants: { backgroundColor: ["responsive", "hover", "focus", "active"] },
  plugins: [require("@tailwindcss/forms")],
};
