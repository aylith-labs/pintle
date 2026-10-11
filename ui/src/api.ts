import type { ProxyRequest, ProxyStats, ProxyTopology } from './types';

async function getJson<T>(path: string, signal?: AbortSignal): Promise<T> {
	const res = await fetch(path, { signal });
	if (!res.ok) throw new Error(`Proxy API request failed (${res.status})`);
	return res.json() as Promise<T>;
}

export async function fetchTopology(signal?: AbortSignal): Promise<ProxyTopology> {
	const data = await getJson<ProxyTopology>('/api/topology', signal);
	if (data.backendHealth) data.backendHealth.receivedAtMs = Date.now();
	return data;
}

export function fetchStats(signal?: AbortSignal): Promise<ProxyStats> {
	return getJson('/api/stats', signal);
}

export function fetchRequests(limit = 50, signal?: AbortSignal): Promise<ProxyRequest[]> {
	return getJson(`/api/requests?limit=${limit}`, signal);
}
