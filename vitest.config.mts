import { defineConfig } from "vitest/config";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
    test: {
        globals: true,
        environment: "node",
        include: ["src/__tests__/**/*.test.ts"],
        coverage: {
            reporter: ["text", "html"],
            include: ["src/lib/**/*.ts"],
            exclude: ["src/lib/**/*.test.ts"],
        },
    },
    resolve: {
        alias: {
            "@": path.resolve(__dirname, "./src"),
        },
    },
});
