import type { ProxyRoute, ProxyStats } from '../types';
import { type HealthState, healthOf } from './health';

export type MatchMode = 'hide' | 'spotlight';
export type SourceKind = ProxyRoute['source'];

export const SOURCE_OPTIONS: SourceKind[] = ['docker', 'static', 'traefik'];

export type FlowFilters = {
	search: string;
	source: SourceKind | null;
	health: HealthState | null;
	matchMode: MatchMode;
};

export const EMPTY_FILTERS: FlowFilters = {
	search: '',
	source: null,
	health: null,
	matchMode: 'hide',
};

export function isFilterActive(filters: FlowFilters): boolean {
	return filters.search.trim() !== '' || filters.source !== null || filters.health !== null;
}

export function matchesRoute(route: ProxyRoute, stats: ProxyStats | null, filters: FlowFilters): boolean {
	const query = filters.search.trim().toLowerCase();
	if (query && !route.hostname.toLowerCase().includes(query)) return false;
	if (filters.source && route.source !== filters.source) return false;
	if (filters.health && healthOf(stats?.hostStats[route.hostname]) !== filters.health) return false;
	return true;
}
