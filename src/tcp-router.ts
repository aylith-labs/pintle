import { createServer, Socket } from 'node:net';
import { TLSSocket } from 'node:tls';
import * as log from './logger';
import { parseSNI } from './sni-router';
import type { TcpRoute } from './types';

type TcpCert = {
	cert: Buffer;
	key: Buffer;
	domain: string; // e.g. "lvh.me" matches *.lvh.me
};

type TcpRouterConfig = {
	port: number;
	certs: TcpCert[];
	getRoutes: () => TcpRoute[];
};

function findCert(hostname: string, certs: TcpCert[]): TcpCert | null {
	for (const c of certs) {
		if (hostname.endsWith(`.${c.domain}`) || hostname === c.domain) {
			return c;
		}
	}
	return null;
}

function startTcpListener(config: TcpRouterConfig): void {
	const server = createServer((clientSocket) => {
		clientSocket.once('data', (data: Buffer) => {
			const sni = parseSNI(data);

			if (!sni) {
				log.warn(`TCP[:${config.port}] No SNI in ClientHello`);
				clientSocket.destroy();
				return;
			}

			const route = config.getRoutes().find((r) => r.listenPort === config.port && r.hostname === sni);

			if (!route) {
				log.warn(`TCP[:${config.port}] No route for ${sni}`);
				clientSocket.destroy();
				return;
			}

			const cert = findCert(sni, config.certs);
			if (!cert) {
				log.warn(`TCP[:${config.port}] No cert for ${sni}`);
				clientSocket.destroy();
				return;
			}

			// Replay peeked data and terminate TLS
			clientSocket.pause();
			clientSocket.unshift(data);

			const tlsSocket = new TLSSocket(clientSocket, {
				isServer: true,
				cert: cert.cert,
				key: cert.key,
			});

			tlsSocket.on('secure', () => {
				const upstream = new Socket();
				upstream.connect(route.targetPort, route.targetHost, () => {
					log.info(`TCP[:${config.port}] ${sni} -> ${route.targetHost}:${route.targetPort}`);
					tlsSocket.pipe(upstream);
					upstream.pipe(tlsSocket);
				});

				upstream.on('error', () => tlsSocket.destroy());
				tlsSocket.on('error', () => upstream.destroy());
			});

			tlsSocket.on('error', (err) => {
				log.warn(`TCP[:${config.port}] TLS error for ${sni}: ${err.message}`);
			});
		});

		clientSocket.on('error', () => {});
	});

	server.listen(config.port, () => {
		log.info(`TCP router listening on :${config.port}`);
	});
}

export function startTcpRouters(params: { ports: number[]; certs: TcpCert[]; getRoutes: () => TcpRoute[] }): void {
	for (const port of params.ports) {
		startTcpListener({
			port,
			certs: params.certs,
			getRoutes: params.getRoutes,
		});
	}
}
