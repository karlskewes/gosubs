export default {
  plugins: {
    "@tailwindcss/postcss": {},
    "@fullhuman/postcss-purgecss": {
      content: ["./**/*.templ",],
      defaultExtractor: (content) => content.match(/[A-Za-z0-9-_:/]+/g) || [],
    },
  },
};
