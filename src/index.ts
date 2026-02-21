import { resolve as resolvePath } from "node:path";
import type { ServerWebSocket } from "bun";
import { getDockerRoutes, initDockerWatcher } from "./docker-watcher";
import * as log from "./logger";
import { handleRequest } from "./proxy";
import { resolve, updateRoutes } from "./router";
import { getStaticRoutes, initStaticRoutes } from "./static-routes";

const HTTPS_PORT = Number.parseInt(process.env.HTTPS_PORT ?? "443", 10);
const HTTP_PORT = Number.parseInt(process.env.HTTP_PORT ?? "80", 10);
const CERT_PATH = resolvePath(import.meta.dir, "../certs/lvh.me.pem");
const KEY_PATH = resolvePath(import.meta.dir, "../certs/lvh.me-key.pem");
const ROUTES_FILE = resolvePath(import.meta.dir, "../routes.yaml");

type WsData = { targetUrl: string; hostname: string };

function rebuildAllRoutes(): void {
	const all = [...getDockerRoutes(), ...getStaticRoutes()];
	updateRoutes(all);
}

// WebSocket proxy: pipe between client and upstream
const wsUpstreams = new Map<ServerWebSocket<WsData>, WebSocket>();

const httpsServer = Bun.serve<WsData>({
	port: HTTPS_PORT,
	tls: {
		cert: Bun.file(CERT_PATH),
		key: Bun.file(KEY_PATH),
	},
	async fetch(req, server) {
		// Handle WebSocket upgrade
		if (req.headers.get("upgrade")?.toLowerCase() === "websocket") {
			const hostname = (req.headers.get("host") ?? "").split(":")[0];
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
const httpServer = Bun.serve({
	port: HTTP_PORT,
	fetch(req) {
		const url = new URL(req.url);
		url.protocol = "https:";
		if (HTTPS_PORT !== 443) {
			url.port = String(HTTPS_PORT);
		}
		return Response.redirect(url.toString(), 301);
	},
});

// Initialize
log.info("local-proxy starting...");

initStaticRoutes(ROUTES_FILE, rebuildAllRoutes);
await initDockerWatcher(rebuildAllRoutes);
rebuildAllRoutes();

log.info(`HTTPS listening on :${httpsServer.port}`);
log.info(`HTTP  listening on :${httpServer.port} (redirect)`);
