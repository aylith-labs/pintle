import { resolve as resolvePath } from "node:path";
import type { ServerWebSocket } from "bun";
import { handleApiRequest } from "./api";
import { BASE_DOMAIN, DASHBOARD_HOST } from "./config";
import { getDockerRoutes, getTraefikTarget, initDockerWatcher } from "./docker-watcher";
import * as log from "./logger";
import { handleRequest } from "./proxy";
import { resolve, updateRoutes } from "./router";
import { startSniRouter } from "./sni-router";
import { getStaticRoutes, initStaticRoutes } from "./static-routes";

const LISTEN_PORT = Number.parseInt(process.env.LISTEN_PORT ?? "9443", 10);
const INTERNAL_HTTPS_PORT = 9444; // Bun HTTPS server (internal, SNI router forwards here)
const HTTP_PORT = Number.parseInt(process.env.HTTP_PORT ?? "9080", 10);
const CERT_PATH = resolvePath(import.meta.dir, `../certs/${BASE_DOMAIN}.pem`);
const KEY_PATH = resolvePath(import.meta.dir, `../certs/${BASE_DOMAIN}-key.pem`);
const CB_CERT_PATH = resolvePath(import.meta.dir, "../certs/_wildcard.internal.example.pem");
const CB_KEY_PATH = resolvePath(import.meta.dir, "../certs/_wildcard.internal.example-key.pem");
const ROUTES_FILE = resolvePath(import.meta.dir, "../routes.yaml");

type WsData = { targetUrl: string; hostname: string };

function rebuildAllRoutes(): void {
	const all = [...getDockerRoutes(), ...getStaticRoutes()];
	updateRoutes(all);
}

// WebSocket proxy: pipe between client and upstream
const wsUpstreams = new Map<ServerWebSocket<WsData>, WebSocket>();

// Internal Bun HTTPS server (receives connections from SNI router for *.lvh.me)
const httpsServer = Bun.serve<WsData>({
	port: INTERNAL_HTTPS_PORT,
	hostname: "127.0.0.1",
	tls: [
		{
			cert: Bun.file(CERT_PATH),
			key: Bun.file(KEY_PATH),
			serverName: `*.${BASE_DOMAIN}`,
		},
		{
			cert: Bun.file(CB_CERT_PATH),
			key: Bun.file(CB_KEY_PATH),
			serverName: "*.internal.example",
		},
	],
	async fetch(req, server) {
		const hostname = (req.headers.get("host") ?? "").split(":")[0];

		// Dashboard API + UI at proxy.<BASE_DOMAIN>
		if (hostname === DASHBOARD_HOST) {
			// Proxy Vite HMR WebSocket
			if (req.headers.get("upgrade")?.toLowerCase() === "websocket") {
				const url = new URL(req.url);
				const targetUrl = `ws://localhost:5175${url.pathname}${url.search}`;
				const upgraded = server.upgrade(req, { data: { targetUrl, hostname } });
				if (upgraded) return undefined;
				return new Response("WebSocket upgrade failed", { status: 400 });
			}
			return handleApiRequest(req);
		}

		// Handle WebSocket upgrade
		if (req.headers.get("upgrade")?.toLowerCase() === "websocket") {
			const url = new URL(req.url);
			const match = resolve(hostname, url.pathname);

			if (match) {
				const targetUrl = `${match.target.replace("http://", "ws://")}${match.rewrittenPath}${url.search}`;
				const upgraded = server.upgrade(req, { data: { targetUrl, hostname } });
				if (upgraded) return undefined;
			}

			return new Response("WebSocket upgrade failed", { status: 400 });
		}

		return handleRequest(req);
	},
	websocket: {
		open(ws) {
			const { targetUrl } = ws.data;

			const upstream = new WebSocket(targetUrl);

			upstream.addEventListener("message", (event) => {
				try {
					ws.send(event.data as string | Buffer);
				} catch {
					// Client disconnected
				}
			});

			upstream.addEventListener("close", () => {
				ws.close();
			});

			upstream.addEventListener("error", () => {
				ws.close();
			});

			wsUpstreams.set(ws, upstream);
		},
		message(ws, message) {
			const upstream = wsUpstreams.get(ws);
			if (upstream?.readyState === WebSocket.OPEN) {
				upstream.send(message);
			}
		},
		close(ws) {
			const upstream = wsUpstreams.get(ws);
			if (upstream) {
				upstream.close();
				wsUpstreams.delete(ws);
			}
		},
	},
});

// HTTP -> HTTPS redirect
Bun.serve({
	port: HTTP_PORT,
	fetch(req) {
		const url = new URL(req.url);
		url.protocol = "https:";
		url.port = ""; // Always redirect to standard port 443 (iptables handles redirect to LISTEN_PORT)
		return Response.redirect(url.toString(), 301);
	},
});

// Initialize routes
log.info("local-proxy starting...");

initStaticRoutes(ROUTES_FILE, rebuildAllRoutes);
await initDockerWatcher(rebuildAllRoutes);
rebuildAllRoutes();

log.info(`Bun HTTPS on :${httpsServer.port} (internal)`);
log.info(`HTTP redirect on :${HTTP_PORT}`);

// SNI router — iptables redirects port 443 → LISTEN_PORT
startSniRouter({
	port: LISTEN_PORT,
	localTarget: { host: "127.0.0.1", port: INTERNAL_HTTPS_PORT },
	forwardTargets: [
		{
			match: (hostname) => hostname.endsWith(".internal.example") || hostname === "internal.example",
			resolve: getTraefikTarget,
			label: "*.internal.example -> Traefik container",
		},
	],
});
