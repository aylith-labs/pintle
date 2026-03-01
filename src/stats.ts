import { DASHBOARD_HOST } from './config';

type RequestRecord = {
	timestamp: number;
	method: string;
	hostname: string;
	path: string;
	target: string;
	status: number;
	durationMs: number;
};

type RouteStats = {
	totalRequests: number;
	errorCount: number;
	avgDurationMs: number;
	lastRequestAt: number;
};

const MAX_BUFFER_SIZE = 1000;
const buffer: RequestRecord[] = [];
const hostStats = new Map<string, RouteStats>();
const edgeStats = new Map<string, RouteStats>();
const startedAt = Date.now();

function updateStats(map: Map<string, RouteStats>, key: string, record: RequestRecord): void {
	const existing = map.get(key);
	if (existing) {
		const total = existing.totalRequests + 1;
		existing.avgDurationMs = (existing.avgDurationMs * existing.totalRequests + record.durationMs) / total;
		existing.totalRequests = total;
		existing.errorCount += record.status >= 400 ? 1 : 0;
		existing.lastRequestAt = record.timestamp;
	} else {
		map.set(key, {
			totalRequests: 1,
			errorCount: record.status >= 400 ? 1 : 0,
			avgDurationMs: record.durationMs,
			lastRequestAt: record.timestamp,
		});
	}
}

export function recordRequest(record: RequestRecord): void {
	// Skip proxy dashboard requests to avoid noise
	if (record.hostname === DASHBOARD_HOST) return;

	buffer.push(record);
	if (buffer.length > MAX_BUFFER_SIZE) {
		buffer.shift();
	}

	updateStats(hostStats, record.hostname, record);
	updateStats(edgeStats, `${record.hostname}->${record.target}`, record);
}

export function getRecentRequests(limit = 50): RequestRecord[] {
	return buffer.slice(-limit).reverse();
}

export function getHostStats(): Record<string, RouteStats> {
	return Object.fromEntries(hostStats);
}

export function getEdgeStats(): Record<string, RouteStats> {
	return Object.fromEntries(edgeStats);
}

export function getUptime(): number {
	return Math.floor((Date.now() - startedAt) / 1000);
}

export function getTotalRequests(): number {
	let total = 0;
	for (const stats of hostStats.values()) {
		total += stats.totalRequests;
	}
	return total;
}
