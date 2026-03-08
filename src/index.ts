import { existsSync, readFileSync } from 'node:fs';
import { resolve as resolvePath } from 'node:path';
import type { ServerWebSocket } from 'bun';
import { handleApiRequest } from './api';
import { BASE_DOMAIN, DASHBOARD_HOST } from './config';
import { getDockerRoutes, getDockerTcpRoutes, getTraefikTarget, initDockerWatcher } from './docker-watcher';
import * as log from './logger';
import { handleRequest } from './proxy';
import { resolve, updateRoutes } from './router';
import { startSniRouter } from './sni-router';
import { getPassthroughDomains, getStaticRoutes, getStaticTcpRoutes, initStaticRoutes } from './static-routes';
import { startTcpRouters } from './tcp-router';

const LISTEN_PORT = Number.parseInt(process.env.LISTEN_PORT ?? '9443', 10);
const HTTP_PORT = Number.parseInt(process.env.HTTP_PORT ?? '9080', 10);
const BUN_INTERNAL_PORT = 9444; // Only used when SNI router is active (passthrough domains)
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
		if (!existsSync(certPath) || !existsSync(keyPath)) {
			log.warn(
				`Passthrough cert missing for *.${pt.domain} (run: mkcert -cert-file certs/${pt.domain}.pem -key-file certs/${pt.domain}-key.pem "*.${pt.domain}")`,
			);
			continue;
		}
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

// --- SNI router (only needed for passthrough domains) ---

const passthroughTargets = getPassthroughDomains().map((pt) => ({
	match: (hostname: string) => hostname.endsWith(`.${pt.domain}`) || hostname === pt.domain,
	resolve: getTraefikTarget,
	label: `*.${pt.domain} -> ${pt.target} container`,
}));
const needsSniRouter = passthroughTargets.length > 0;
const httpsPort = needsSniRouter ? BUN_INTERNAL_PORT : LISTEN_PORT;
const httpsHostname = needsSniRouter ? '127.0.0.1' : undefined;

// --- HTTPS server ---

const httpsServer = Bun.serve<WsData>({
	port: httpsPort,
	hostname: httpsHostname,
	idleTimeout: 120,
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
				const viteWsUrl = (process.env.VITE_DEV_URL ?? 'http://localhost:5175').replace('http://', 'ws://');
			const targetUrl = `${viteWsUrl}${url.pathname}${url.search}`;
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

log.info(`Bun HTTPS on :${httpsServer.port}${needsSniRouter ? ' (internal, behind SNI router)' : ''}`);
log.info(`HTTP redirect on :${HTTP_PORT}`);

if (needsSniRouter) {
	startSniRouter({
		port: LISTEN_PORT,
		localTarget: { host: '127.0.0.1', port: BUN_INTERNAL_PORT },
		forwardTargets: passthroughTargets,
	});
}

// --- TCP routers (Redis, PostgreSQL, MySQL — TLS termination + plain TCP to container) ---

function buildTcpCerts(): { cert: Buffer; key: Buffer; domain: string }[] {
	const certs: { cert: Buffer; key: Buffer; domain: string }[] = [];

	// Base domain cert (*.lvh.me)
	if (existsSync(CERT_PATH) && existsSync(KEY_PATH)) {
		certs.push({ cert: readFileSync(CERT_PATH), key: readFileSync(KEY_PATH), domain: BASE_DOMAIN });
	}

	// Passthrough domain certs (*.internal.example, etc.)
	for (const pt of getPassthroughDomains()) {
		const certPath = resolvePath(CERTS_DIR, `${pt.domain}.pem`);
		const keyPath = resolvePath(CERTS_DIR, `${pt.domain}-key.pem`);
		if (existsSync(certPath) && existsSync(keyPath)) {
			certs.push({ cert: readFileSync(certPath), key: readFileSync(keyPath), domain: pt.domain });
		}
	}

	return certs;
}

const allTcpRoutes = () => [...getDockerTcpRoutes(), ...getStaticTcpRoutes()];
const tcpRoutes = allTcpRoutes();

if (tcpRoutes.length > 0) {
	const activePorts = [...new Set(tcpRoutes.map((r) => r.listenPort))];
	startTcpRouters({
		ports: activePorts,
		certs: buildTcpCerts(),
		getRoutes: allTcpRoutes,
	});
} else {
	log.info('No TCP routes discovered, skipping TCP routers');
}
