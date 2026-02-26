import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
	plugins: [react(), tailwindcss()],
	server: {
		port: 5175,
		strictPort: true,
		allowedHosts: ["proxy.lvh.me"],
		hmr: {
			// Connect HMR directly to Vite, bypassing the proxy's WebSocket relay
			// Prevents ECONNRESET crashes when browser disconnects
			host: "localhost",
			port: 5175,
			protocol: "ws",
		},
	},
	build: {
		outDir: "dist",
	},
});
