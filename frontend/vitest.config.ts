import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    // The helpers under test are runtime-agnostic; Node 20 supplies
    // fetch / TextEncoder / TextDecoder / ReadableStream as globals.
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
