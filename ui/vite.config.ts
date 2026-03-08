import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [react(), tailwindcss()],
	server: {
		host: '0.0.0.0',
		port: 5175,
		strictPort: true,
		allowedHosts: true,
		hmr: {
			// HMR goes through the proxy at proxy.lvh.me
			host: `proxy.${process.env.BASE_DOMAIN ?? 'lvh.me'}`,
			port: 443,
			protocol: 'wss',
		},
	},
	build: {
		outDir: 'dist',
	},
});
