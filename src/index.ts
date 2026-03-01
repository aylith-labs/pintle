import { resolve as resolvePath } from 'node:path';
import type { ServerWebSocket } from 'bun';
import { handleApiRequest } from './api';
import { BASE_DOMAIN, DASHBOARD_HOST } from './config';
import { getDockerRoutes, getTraefikTarget, initDockerWatcher } from './docker-watcher';
import * as log from './logger';
import { handleRequest } from './proxy';
import { resolve, updateRoutes } from './router';
import { startSniRouter } from './sni-router';
import { getPassthroughDomains, getStaticRoutes, initStaticRoutes } from './static-routes';

const LISTEN_PORT = Number.parseInt(process.env.LISTEN_PORT ?? '9443', 10);
const INTERNAL_HTTPS_PORT = 9444; // Bun HTTPS server (internal, SNI router forwards here)
const HTTP_PORT = Number.parseInt(process.env.HTTP_PORT ?? '9080', 10);
const CERTS_DIR = resolvePath(import.meta.dir, '../certs');
const CERT_PATH = resolvePath(CERTS_DIR, `${BASE_DOMAIN}.pem`);
const KEY_PATH = resolvePath(CERTS_DIR, `${BASE_DOMAIN}-key.pem`);
const ROUTES_FILE = resolvePath(import.meta.dir, '../routes.yaml');

type WsData = { targetUrl: string; hostname: string };

function rebuildAllRoutes(): void {
	const all = [...getDockerRoutes(), ...getStaticRoutes()];
	updateRoutes(all);
}

// --- Initialize routes (must happen before server starts) ---

log.info('local-proxy starting...');

initStaticRoutes(ROUTES_FILE, rebuildAllRoutes);
await initDockerWatcher(rebuildAllRoutes);
rebuildAllRoutes();

// --- Build TLS entries for passthrough domains ---

function buildPassthroughTls(): {
	cert: ReturnType<typeof Bun.file>;
	key: ReturnType<typeof Bun.file>;
	serverName: string;
}[] {
	const entries: { cert: ReturnType<typeof Bun.file>; key: ReturnType<typeof Bun.file>; serverName: string }[] = [];
	for (const pt of getPassthroughDomains()) {
		const certPath = resolvePath(CERTS_DIR, `${pt.domain}.pem`);
		const keyPath = resolvePath(CERTS_DIR, `${pt.domain}-key.pem`);
		entries.push({
			cert: Bun.file(certPath),
			key: Bun.file(keyPath),
			serverName: `*.${pt.domain}`,
		});
	}
	return entries;
}

// --- WebSocket proxy ---

const wsUpstreams = new Map<ServerWebSocket<WsData>, WebSocket>();

// --- HTTPS server (receives connections from SNI router) ---

const httpsServer = Bun.serve<WsData>({
	port: INTERNAL_HTTPS_PORT,
	hostname: '127.0.0.1',
	tls: [
		{
			cert: Bun.file(CERT_PATH),
			key: Bun.file(KEY_PATH),
			serverName: `*.${BASE_DOMAIN}`,
		},
		...buildPassthroughTls(),
	],
	async fetch(req, server) {
		const hostname = (req.headers.get('host') ?? '').split(':')[0];

		// Dashboard API + UI at proxy.<BASE_DOMAIN>
		if (hostname === DASHBOARD_HOST) {
			// Proxy Vite HMR WebSocket
			if (req.headers.get('upgrade')?.toLowerCase() === 'websocket') {
				const url = new URL(req.url);
				const targetUrl = `ws://localhost:5175${url.pathname}${url.search}`;
				const upgraded = server.upgrade(req, { data: { targetUrl, hostname } });
				if (upgraded) return undefined;
				return new Response('WebSocket upgrade failed', { status: 400 });
			}
			return handleApiRequest(req);
		}

		// Handle WebSocket upgrade
		if (req.headers.get('upgrade')?.toLowerCase() === 'websocket') {
			const url = new URL(req.url);
			const match = resolve(hostname, url.pathname);

			if (match) {
				const targetUrl = `${match.target.replace('http://', 'ws://')}${match.rewrittenPath}${url.search}`;
				const upgraded = server.upgrade(req, { data: { targetUrl, hostname } });
				if (upgraded) return undefined;
			}

			return new Response('WebSocket upgrade failed', { status: 400 });
		}

		return handleRequest(req);
	},
	websocket: {
		open(ws) {
			const { targetUrl } = ws.data;

			const upstream = new WebSocket(targetUrl);

			upstream.addEventListener('message', (event) => {
				try {
					ws.send(event.data as string | Buffer);
				} catch {
					// Client disconnected
				}
			});

			upstream.addEventListener('close', () => {
				ws.close();
			});

			upstream.addEventListener('error', () => {
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

// --- HTTP -> HTTPS redirect ---

Bun.serve({
	port: HTTP_PORT,
	fetch(req) {
		const url = new URL(req.url);
		url.protocol = 'https:';
		url.port = ''; // Always redirect to standard port 443 (iptables handles redirect to LISTEN_PORT)
		return Response.redirect(url.toString(), 301);
	},
});

log.info(`Bun HTTPS on :${httpsServer.port} (internal)`);
log.info(`HTTP redirect on :${HTTP_PORT}`);

// --- SNI router (iptables/pfctl redirects port 443 → LISTEN_PORT) ---

const forwardTargets = getPassthroughDomains().map((pt) => ({
	match: (hostname: string) => hostname.endsWith(`.${pt.domain}`) || hostname === pt.domain,
	resolve: getTraefikTarget, // Currently only "traefik" target is supported
	label: `*.${pt.domain} -> ${pt.target} container`,
}));

startSniRouter({
	port: LISTEN_PORT,
	localTarget: { host: '127.0.0.1', port: INTERNAL_HTTPS_PORT },
	forwardTargets,
});
