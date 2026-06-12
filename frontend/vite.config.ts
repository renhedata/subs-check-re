import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { nitro } from "nitro/vite";
import { defineConfig } from "vite";

export default defineConfig({
	plugins: [tailwindcss(), nitro(), tanstackStart(), viteReact()],
	resolve: {
		alias: {
			"@": path.resolve(__dirname, "./src"),
		},
		dedupe: [
			"react",
			"react-dom",
			"@tanstack/react-router",
			"@tanstack/router-core",
		],
	},
	server: {
		port: 3001,
	},
	ssr: {
		// Force these packages to be bundled for SSR to avoid duplicate React instances
		// caused by bun's symlink-based module resolution in .bun/ cache
		noExternal: [
			"@tanstack/react-query",
			"@tanstack/react-router",
			"@tanstack/react-start",
		],
	},
});
