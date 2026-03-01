import Docker from 'dockerode';
import * as log from './logger';
import type { Route } from './types';

const docker = new Docker();

let currentRoutes: Route[] = [];
let onChange: (() => void) | null = null;
let rebuildTimer: ReturnType<typeof setTimeout> | null = null;
let traefikIp: string | null = null;

const NETWORK_NAME = process.env.DOCKER_NETWORK ?? 'traefik';

function parseProxyLabels(labels: Record<string, string>): {
	hosts: string[];
	port: number | null;
	path: string;
	strip: boolean;
} | null {
	const hostLabel = labels['local-proxy.host'];
	if (!hostLabel) return null;

	return {
		hosts: hostLabel.split(',').map((h) => h.trim()),
		port: labels['local-proxy.port'] ? Number.parseInt(labels['local-proxy.port'], 10) : null,
		path: labels['local-proxy.path'] ?? '/',
		strip: labels['local-proxy.strip'] === 'true',
	};
}

function parseTraefikLabels(labels: Record<string, string>): {
	hosts: string[];
	port: number | null;
	path: string;
	strip: boolean;
} | null {
	if (labels['traefik.enable'] !== 'true') return null;

	// Find router rule: traefik.http.routers.<NAME>.rule
	let ruleValue: string | null = null;
	for (const [key, value] of Object.entries(labels)) {
		if (/^traefik\.http\.routers\..+\.rule$/.test(key)) {
			ruleValue = value;
			break;
		}
	}
	if (!ruleValue) return null;

	// Parse Host(`...`) from rule
	const hostMatches = [...ruleValue.matchAll(/Host\(`([^`]+)`\)/g)];
	if (hostMatches.length === 0) return null;
	const hosts = hostMatches.map((m) => m[1]);

	// Parse optional PathPrefix(`...`)
	const pathMatch = ruleValue.match(/PathPrefix\(`([^`]+)`\)/);
	const path = pathMatch ? pathMatch[1] : '/';

	// Find port: traefik.http.services.<NAME>.loadbalancer.server.port
	let port: number | null = null;
	for (const [key, value] of Object.entries(labels)) {
		if (/^traefik\.http\.services\..+\.loadbalancer\.server\.port$/.test(key)) {
			port = Number.parseInt(value, 10);
			break;
		}
	}

	// Check for stripprefix middleware
	let strip = false;
	for (const key of Object.keys(labels)) {
		if (/^traefik\.http\.middlewares\..+\.stripprefix\.prefixes$/.test(key)) {
			strip = true;
			break;
		}
	}

	return { hosts, port, path, strip };
}

function parseCaddyLabels(labels: Record<string, string>): {
	hosts: string[];
	port: number | null;
	path: string;
	strip: boolean;
}[] {
	// Caddy labels use `caddy` or `caddy_N` prefixes for multi-domain
	const configs = new Map<string, { host: string | null; port: number | null; path: string; strip: boolean }>();

	// Find all caddy prefixes (caddy, caddy_0, caddy_1, ...)
	for (const [key, value] of Object.entries(labels)) {
		const prefixMatch = key.match(/^(caddy(?:_\d+)?)$/);
		if (prefixMatch) {
			const prefix = prefixMatch[1];
			configs.set(prefix, { host: value, port: null, path: '/', strip: false });
		}
	}

	if (configs.size === 0) return [];

	// Parse directives for each prefix
	for (const [key, value] of Object.entries(labels)) {
		for (const [prefix, config] of configs) {
			// Port from reverse_proxy: {{upstreams PORT}} or {{upstreams http PORT}}
			if (key === `${prefix}.reverse_proxy`) {
				const portMatch = value.match(/\{\{upstreams(?:\s+https?)?\s+(\d+)\}\}/);
				if (portMatch) {
					config.port = Number.parseInt(portMatch[1], 10);
				}
			}

			// Path with strip: handle_path
			if (key === `${prefix}.handle_path`) {
				config.path = value.replace(/\/?\*$/, '') || '/';
				config.strip = true;
			}

			// Path without strip: handle
			if (key === `${prefix}.handle` && config.path === '/') {
				config.path = value.replace(/\/?\*$/, '') || '/';
			}
		}
	}

	// Convert to results, filtering out entries without a host
	const results: { hosts: string[]; port: number | null; path: string; strip: boolean }[] = [];
	for (const config of configs.values()) {
		if (!config.host) continue;
		results.push({
			hosts: [config.host],
			port: config.port,
			path: config.path,
			strip: config.strip,
		});
	}

	return results;
}

function resolveContainerRoute(
	containerInfo: Docker.ContainerInfo,
	parsed: { hosts: string[]; port: number | null; path: string; strip: boolean },
	source: 'docker' | 'traefik' | 'caddy',
): Route[] {
	const networks = containerInfo.NetworkSettings?.Networks ?? {};
	const networkInfo = networks[NETWORK_NAME];
	const name = containerInfo.Names[0]?.replace(/^\//, '') ?? 'unknown';

	if (!networkInfo?.IPAddress) {
		log.error(`Container ${name} (${source}) has no IP on network '${NETWORK_NAME}'`);
		return [];
	}

	let port = parsed.port;
	if (!port && containerInfo.Ports.length > 0) {
		port = containerInfo.Ports[0].PrivatePort;
	}
	if (!port) {
		log.error(`Container ${name} (${source}) has no port configured`);
		return [];
	}

	const target = `http://${networkInfo.IPAddress}:${port}`;

	return parsed.hosts.map((hostname) => ({
		hostname,
		path: parsed.path,
		target,
		stripPath: parsed.strip,
		source,
		containerName: name,
	}));
}

async function discoverProxyRoutes(): Promise<Route[]> {
	const containers = await docker.listContainers({
		filters: { label: ['local-proxy.host'] },
	});

	const routes: Route[] = [];
	for (const containerInfo of containers) {
		const parsed = parseProxyLabels(containerInfo.Labels);
		if (!parsed) continue;
		routes.push(...resolveContainerRoute(containerInfo, parsed, 'docker'));
	}
	return routes;
}

async function discoverTraefikRoutes(excludeContainers: Set<string>): Promise<Route[]> {
	const containers = await docker.listContainers({
		filters: { label: ['traefik.enable=true'] },
	});

	const routes: Route[] = [];
	for (const containerInfo of containers) {
		const name = containerInfo.Names[0]?.replace(/^\//, '') ?? 'unknown';

		// Skip containers already handled by proxy.* labels
		if (excludeContainers.has(name)) continue;

		// Skip the Traefik container itself
		if (containerInfo.Image.includes('traefik')) continue;

		const parsed = parseTraefikLabels(containerInfo.Labels);
		if (!parsed) continue;
		routes.push(...resolveContainerRoute(containerInfo, parsed, 'traefik'));
	}
	return routes;
}

async function discoverCaddyRoutes(excludeContainers: Set<string>): Promise<Route[]> {
	// List all containers and check for caddy/caddy_N labels
	const containers = await docker.listContainers();

	const routes: Route[] = [];
	for (const containerInfo of containers) {
		const name = containerInfo.Names[0]?.replace(/^\//, '') ?? 'unknown';
		if (excludeContainers.has(name)) continue;

		const hasCaddyLabel = Object.keys(containerInfo.Labels).some((k) => /^caddy(_\d+)?$/.test(k));
		if (!hasCaddyLabel) continue;

		const parsedList = parseCaddyLabels(containerInfo.Labels);
		for (const parsed of parsedList) {
			routes.push(...resolveContainerRoute(containerInfo, parsed, 'caddy'));
		}
	}
	return routes;
}

async function discoverAllRoutes(): Promise<Route[]> {
	// Proxy labels take precedence, then traefik, then caddy
	const proxyRoutes = await discoverProxyRoutes();

	const proxyContainers = new Set(proxyRoutes.map((r) => r.containerName).filter(Boolean) as string[]);

	const traefikRoutes = await discoverTraefikRoutes(proxyContainers);

	const handledContainers = new Set([
		...proxyContainers,
		...(traefikRoutes.map((r) => r.containerName).filter(Boolean) as string[]),
	]);

	const caddyRoutes = await discoverCaddyRoutes(handledContainers);

	const counts = [
		proxyRoutes.length > 0 ? `${proxyRoutes.length} proxy` : null,
		traefikRoutes.length > 0 ? `${traefikRoutes.length} traefik` : null,
		caddyRoutes.length > 0 ? `${caddyRoutes.length} caddy` : null,
	].filter(Boolean);

	if (counts.length > 1) {
		log.info(`Discovered ${counts.join(' + ')} route(s)`);
	}

	return [...proxyRoutes, ...traefikRoutes, ...caddyRoutes];
}

function scheduleRebuild(): void {
	if (rebuildTimer) clearTimeout(rebuildTimer);
	rebuildTimer = setTimeout(async () => {
		try {
			currentRoutes = await discoverAllRoutes();
			await discoverTraefik();
			onChange?.();
		} catch (err) {
			log.error('Failed to rebuild Docker routes', err);
		}
	}, 300);
}

export function getDockerRoutes(): Route[] {
	return currentRoutes;
}

export function getTraefikTarget(): { host: string; port: number } | null {
	if (!traefikIp) return null;
	return { host: traefikIp, port: 443 };
}

async function discoverTraefik(): Promise<void> {
	try {
		const containers = await docker.listContainers();
		for (const c of containers) {
			if (c.Image.includes('traefik')) {
				const networks = c.NetworkSettings?.Networks ?? {};
				const networkInfo = networks[NETWORK_NAME];
				if (networkInfo?.IPAddress) {
					const oldIp = traefikIp;
					traefikIp = networkInfo.IPAddress;
					if (oldIp !== traefikIp) {
						log.info(`Traefik container IP: ${traefikIp} (network '${NETWORK_NAME}')`);
					}
					return;
				}
			}
		}
		if (traefikIp) {
			log.info('Traefik container not found, clearing cached IP');
			traefikIp = null;
		}
	} catch {
		// Docker query failed, keep existing cached IP
	}
}

export async function initDockerWatcher(onUpdate: () => void): Promise<void> {
	onChange = onUpdate;

	// Initial discovery
	try {
		currentRoutes = await discoverAllRoutes();
		await discoverTraefik();
		log.info(`Discovered ${currentRoutes.length} Docker route(s) on network '${NETWORK_NAME}'`);
	} catch (err) {
		log.error('Failed initial Docker discovery', err);
	}

	// Watch for container events
	try {
		const stream = await docker.getEvents({
			filters: {
				type: ['container'],
				event: ['start', 'stop', 'die', 'destroy'],
			},
		});

		stream.on('data', () => {
			scheduleRebuild();
		});

		stream.on('error', (err) => {
			log.error('Docker event stream error', err);
			// Reconnect after delay
			setTimeout(() => initDockerWatcher(onUpdate), 5000);
		});

		log.info('Watching Docker events for container changes');
	} catch (err) {
		log.error('Failed to watch Docker events', err);
	}
}
