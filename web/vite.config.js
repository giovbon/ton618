import { defineConfig } from "vite";
import preact from "@preact/preset-vite";
import tailwindcss from "@tailwindcss/vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [preact(), tailwindcss()],
  resolve: {
    alias: {
      react: "preact/compat",
      "react-dom": "preact/compat",
      "react/jsx-runtime": "preact/jsx-runtime",
    },
  },
  define: {
    "process.env": {},
  },
  server: {
    proxy: {
      "/api": "http://localhost:6180",
      "/docs": "http://localhost:6180",
    },
  },
  build: {
    chunkSizeWarningLimit: 1000,
    cssCodeSplit: false,
    sourcemap: true,
  },
  test: {
    testTimeout: 15000,
    environment: "jsdom",
    setupFiles: ["./src/test/setup.jsx"],
    alias: {
      react: "preact/compat",
      "react-dom": "preact/compat",
      "react/jsx-runtime": "preact/jsx-runtime",
    },
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      exclude: ["node_modules/", "src/test/setup.jsx"],
    },
  },
});
