import * as log from './logger';
import type { Route } from './types';

let routeMap = new Map<string, Route[]>();

export function getAllRoutes(): Route[] {
	const all: Route[] = [];
	for (const routes of routeMap.values()) {
		all.push(...routes);
	}
	return all;
}

export function updateRoutes(routes: Route[]): void {
	const prev = routeMap;
	const next = new Map<string, Route[]>();

	for (const route of routes) {
		const existing = next.get(route.hostname) ?? [];
		existing.push(route);
		next.set(route.hostname, existing);
	}

	// Sort each hostname's routes by path specificity (longest first)
	for (const [hostname, hostRoutes] of next) {
		hostRoutes.sort((a, b) => b.path.length - a.path.length);
		next.set(hostname, hostRoutes);
	}

	// Log changes
	const prevSet = new Set<string>();
	for (const routes of prev.values()) {
		for (const r of routes) {
			prevSet.add(`${r.hostname}${r.path}->${r.target}`);
		}
	}

	const nextSet = new Set<string>();
	for (const r of routes) {
		nextSet.add(`${r.hostname}${r.path}->${r.target}`);
	}

	for (const r of routes) {
		const key = `${r.hostname}${r.path}->${r.target}`;
		if (!prevSet.has(key)) {
			log.routeChange('add', r.hostname, r.path, r.target);
		}
	}

	for (const prevRoutes of prev.values()) {
		for (const r of prevRoutes) {
			const key = `${r.hostname}${r.path}->${r.target}`;
			if (!nextSet.has(key)) {
				log.routeChange('remove', r.hostname, r.path, r.target);
			}
		}
	}

	routeMap = next;
}

export function resolve(hostname: string, path: string): { target: string; rewrittenPath: string } | null {
	const routes = routeMap.get(hostname);
	if (!routes) return null;

	for (const route of routes) {
		if (route.path === '/' || path === route.path || path.startsWith(`${route.path}/`)) {
			const rewrittenPath = route.stripPath ? path.slice(route.path.length) || '/' : path;
			return { target: route.target, rewrittenPath };
		}
	}

	return null;
}
