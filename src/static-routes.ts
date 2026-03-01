import { readFileSync, watchFile } from 'node:fs';
import { parse } from 'yaml';
import { HOST_ADDRESS } from './config';
import * as log from './logger';
import type { PassthroughConfig, Route, StaticRouteConfig, StaticTcpRouteConfig, TcpRoute } from './types';

function resolveTarget(target: string | number): string {
	if (typeof target === 'number') return `http://${HOST_ADDRESS}:${target}`;
	if (/^\d+$/.test(target)) return `http://${HOST_ADDRESS}:${target}`;
	return target;
}

let currentRoutes: Route[] = [];
let currentTcpRoutes: TcpRoute[] = [];
let currentPassthrough: PassthroughConfig[] = [];
let onChange: (() => void) | null = null;

type RoutesFile = {
	routes?: StaticRouteConfig[];
	passthrough?: PassthroughConfig[];
	tcp?: StaticTcpRouteConfig[];
} | null;

function resolveTcpTarget(target: string | number): { host: string; port: number } {
	if (typeof target === 'number') return { host: HOST_ADDRESS, port: target };
	if (/^\d+$/.test(target)) return { host: HOST_ADDRESS, port: Number.parseInt(target, 10) };
	const [host, port] = target.split(':');
	return { host, port: Number.parseInt(port, 10) };
}

function loadFile(filePath: string): { routes: Route[]; passthrough: PassthroughConfig[]; tcpRoutes: TcpRoute[] } {
	try {
		const content = readFileSync(filePath, 'utf-8');
		const parsed = parse(content) as RoutesFile;

		const routes = (parsed?.routes ?? []).map((r) => ({
			hostname: r.host,
			path: r.path ?? '/',
			target: resolveTarget(r.target),
			stripPath: r.strip ?? false,
			source: 'static' as const,
		}));

		const passthrough = parsed?.passthrough ?? [];

		const tcpRoutes: TcpRoute[] = (parsed?.tcp ?? []).map((t) => {
			const resolved = resolveTcpTarget(t.target);
			return {
				hostname: t.host,
				targetHost: resolved.host,
				targetPort: resolved.port,
				listenPort: t.listen,
				source: 'static' as const,
			};
		});

		return { routes, passthrough, tcpRoutes };
	} catch (err) {
		log.error(`Failed to load routes file: ${filePath}`, err);
		return { routes: [], passthrough: [], tcpRoutes: [] };
	}
}

export function getStaticRoutes(): Route[] {
	return currentRoutes;
}

export function getStaticTcpRoutes(): TcpRoute[] {
	return currentTcpRoutes;
}

export function getPassthroughDomains(): PassthroughConfig[] {
	return currentPassthrough;
}

export function initStaticRoutes(filePath: string, onUpdate: () => void): void {
	onChange = onUpdate;
	const loaded = loadFile(filePath);
	currentRoutes = loaded.routes;
	currentTcpRoutes = loaded.tcpRoutes;
	currentPassthrough = loaded.passthrough;
	log.info(
		`Loaded ${currentRoutes.length} static route(s), ${currentTcpRoutes.length} TCP route(s), ${currentPassthrough.length} passthrough domain(s)`,
	);

	watchFile(filePath, { interval: 1000 }, () => {
		log.info('Routes file changed, reloading...');
		const reloaded = loadFile(filePath);
		currentRoutes = reloaded.routes;
		currentTcpRoutes = reloaded.tcpRoutes;
		currentPassthrough = reloaded.passthrough;
		onChange?.();
	});
}
