import type { ProxyRoute, RouteStats } from '../types';

export type GroupingMode = 'none' | 'domain' | 'source' | 'prefix';

export const GROUPING_MODES: GroupingMode[] = ['none', 'domain', 'source', 'prefix'];

export const GROUPING_THRESHOLD = 5;

const OTHER_KEY = '__other';

export type ServiceGroup = {
	key: string;
	label: string;
	routes: ProxyRoute[];
};

export function formatCount(n: number): string {
	if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
	return String(n);
}

function looksLikeRegex(hostname: string): boolean {
	return /[\^$()[\]\\|]/.test(hostname) || hostname.includes('.*') || hostname.includes('.+');
}

// Turn a Traefik HostRegexp pattern into a glob-ish hostname so it groups
// by its real base domain, e.g. `^[^.]+[.]example[.]com$` -> `*.example.com`.
// Plain hostnames pass through unchanged.
export function normalizeHostname(hostname: string): string {
	if (!looksLikeRegex(hostname)) return hostname;
	const collapsed = hostname
		.trim()
		.replace(/^\(\?[a-z]+\)/, '') // inline flags like (?i)
		.replace(/^\^/, '')
		.replace(/\$$/, '')
		.replace(/\\\./g, '.') // escaped dots
		.replace(/\[\.\]/g, '.') // char-class dots
		.replace(/\[\^.\]\+/g, '*') // single-label wildcard [^.]+
		.replace(/\[\^.\]\*/g, '*')
		.replace(/\.\*/g, '*') // greedy wildcards
		.replace(/\.\+/g, '*');
	// Any leftover regex-y label becomes a wildcard so it can't pollute the key.
	const labels = collapsed.split('.').map((label) => (/^[a-z0-9*-]+$/i.test(label) ? label : '*'));
	return labels.join('.');
}

function baseDomain(hostname: string): string {
	const parts = normalizeHostname(hostname).split('.');
	if (parts.length <= 2) return parts.join('.');
	return parts.slice(-2).join('.');
}

function hostPrefix(hostname: string): string {
	const firstLabel = normalizeHostname(hostname).split('.')[0];
	return firstLabel.split('-')[0];
}

function groupKey(route: ProxyRoute, mode: GroupingMode): string {
	if (mode === 'source') return route.source;
	if (mode === 'prefix') return hostPrefix(route.hostname);
	return baseDomain(route.hostname);
}

export function groupRoutes(routes: ProxyRoute[], mode: GroupingMode): ServiceGroup[] {
	const buckets = new Map<string, ProxyRoute[]>();
	for (const route of routes) {
		const key = groupKey(route, mode);
		const bucket = buckets.get(key);
		if (bucket) bucket.push(route);
		else buckets.set(key, [route]);
	}

	// Prefix mode: merge size-1 buckets into a single "other" bucket so a
	// long tail of unique prefixes doesn't degenerate into ungrouped nodes.
	if (mode === 'prefix') {
		const singles: ProxyRoute[] = [];
		for (const [key, bucket] of buckets) {
			if (bucket.length === 1) {
				singles.push(bucket[0]);
				buckets.delete(key);
			}
		}
		if (singles.length > 0) {
			buckets.set(OTHER_KEY, singles);
		}
	}

	const groups: ServiceGroup[] = [];
	for (const [key, bucket] of buckets) {
		bucket.sort((left, right) => left.hostname.localeCompare(right.hostname));
		groups.push({ key, label: key === OTHER_KEY ? 'other' : key, routes: bucket });
	}

	groups.sort((left, right) => {
		if (left.key === OTHER_KEY) return 1;
		if (right.key === OTHER_KEY) return -1;
		return left.label.localeCompare(right.label);
	});

	return groups;
}

export function aggregateStats(
	hostnames: string[],
	hostStats: Record<string, RouteStats> | undefined,
): RouteStats | undefined {
	if (!hostStats) return undefined;

	let totalRequests = 0;
	let errorCount = 0;
	let durationWeighted = 0;
	let lastRequestAt = 0;
	let hasStats = false;

	for (const hostname of hostnames) {
		const stat = hostStats[hostname];
		if (!stat) continue;
		hasStats = true;
		totalRequests += stat.totalRequests;
		errorCount += stat.errorCount;
		durationWeighted += stat.avgDurationMs * stat.totalRequests;
		lastRequestAt = Math.max(lastRequestAt, stat.lastRequestAt);
	}

	if (!hasStats) return undefined;

	return {
		totalRequests,
		errorCount,
		avgDurationMs: totalRequests > 0 ? durationWeighted / totalRequests : 0,
		lastRequestAt,
	};
}
