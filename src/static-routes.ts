import { readFileSync, watchFile } from 'node:fs';
import { parse } from 'yaml';
import * as log from './logger';
import type { PassthroughConfig, Route, StaticRouteConfig } from './types';

let currentRoutes: Route[] = [];
let currentPassthrough: PassthroughConfig[] = [];
let onChange: (() => void) | null = null;

type RoutesFile = {
	routes?: StaticRouteConfig[];
	passthrough?: PassthroughConfig[];
} | null;

function loadFile(filePath: string): { routes: Route[]; passthrough: PassthroughConfig[] } {
	try {
		const content = readFileSync(filePath, 'utf-8');
		const parsed = parse(content) as RoutesFile;

		const routes = (parsed?.routes ?? []).map((r) => ({
			hostname: r.host,
			path: r.path ?? '/',
			target: r.target,
			stripPath: r.strip ?? false,
			source: 'static' as const,
		}));

		const passthrough = parsed?.passthrough ?? [];

		return { routes, passthrough };
	} catch (err) {
		log.error(`Failed to load routes file: ${filePath}`, err);
		return { routes: [], passthrough: [] };
	}
}

export function getStaticRoutes(): Route[] {
	return currentRoutes;
}

export function getPassthroughDomains(): PassthroughConfig[] {
	return currentPassthrough;
}

export function initStaticRoutes(filePath: string, onUpdate: () => void): void {
	onChange = onUpdate;
	const loaded = loadFile(filePath);
	currentRoutes = loaded.routes;
	currentPassthrough = loaded.passthrough;
	log.info(`Loaded ${currentRoutes.length} static route(s), ${currentPassthrough.length} passthrough domain(s)`);

	watchFile(filePath, { interval: 1000 }, () => {
		log.info('Routes file changed, reloading...');
		const reloaded = loadFile(filePath);
		currentRoutes = reloaded.routes;
		currentPassthrough = reloaded.passthrough;
		onChange?.();
	});
}
