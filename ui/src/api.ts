import type { ProxyRequest, ProxyStats, ProxyTopology } from './types';

export async function fetchTopology(): Promise<ProxyTopology> {
	const res = await fetch('/api/topology');
	if (!res.ok) throw new Error('Could not refresh proxy topology.');
	const data: ProxyTopology = await res.json();
	if (data.backendHealth) data.backendHealth.receivedAtMs = Date.now();
	return data;
}

export async function fetchStats(): Promise<ProxyStats> {
	const res = await fetch('/api/stats');
	return res.json();
}

export async function fetchRequests(limit = 50): Promise<ProxyRequest[]> {
	const res = await fetch(`/api/requests?limit=${limit}`);
	return res.json();
}
