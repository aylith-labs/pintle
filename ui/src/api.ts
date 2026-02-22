import type { ProxyRequest, ProxyStats, ProxyTopology } from "./types";

export async function fetchTopology(): Promise<ProxyTopology> {
	const res = await fetch("/api/topology");
	return res.json();
}

export async function fetchStats(): Promise<ProxyStats> {
	const res = await fetch("/api/stats");
	return res.json();
}

export async function fetchRequests(limit = 50): Promise<ProxyRequest[]> {
	const res = await fetch(`/api/requests?limit=${limit}`);
	return res.json();
}
