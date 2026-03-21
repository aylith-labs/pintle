import { type Edge, MarkerType, type Node } from '@xyflow/react';
import { useMemo } from 'react';
import type { ProxyStats, ProxyTopology, RouteStats } from '../types';

const BASE_COL = [0, 220, 460, 720];
const BASE_ROW_HEIGHT = 90;

function edgeStyle(stats: RouteStats | undefined): { stroke: string; strokeWidth: number } {
	if (!stats) return { stroke: '#6366f1', strokeWidth: 1.5 };
	const errorRate = stats.errorCount / Math.max(stats.totalRequests, 1);
	if (errorRate > 0.5) return { stroke: '#ef4444', strokeWidth: 2 };
	if (errorRate > 0.1) return { stroke: '#f59e0b', strokeWidth: 2 };
	return { stroke: '#22c55e', strokeWidth: 2 };
}

function edgeLabel(stats: RouteStats | undefined): string {
	if (!stats || stats.totalRequests === 0) return '';
	return `${stats.totalRequests} req`;
}

export function useFlowLayout(topology: ProxyTopology | null, stats: ProxyStats | null, scale = 1) {
	const isDark = document.documentElement.classList.contains('dark');

	return useMemo(() => {
		if (!topology) return { nodes: [] as Node[], edges: [] as Edge[] };

		const COL = BASE_COL.map((c) => c * scale);
		const ROW_HEIGHT = BASE_ROW_HEIGHT * scale;
		const labelBg = isDark ? '#09090b' : '#fafafa';

		const nodes: Node[] = [];
		const edges: Edge[] = [];
		const ROW1 = 60 * scale;
		const ROW2 = 200 * scale;
		const labelFontSize = 10 * scale;

		// Col 0: Browser
		nodes.push({
			id: 'browser',
			type: 'infra',
			position: { x: COL[0], y: ROW1 },
			data: { label: 'Browser', sublabel: 'HTTPS', port: 443, icon: 'globe' },
			draggable: true,
		});

		// Col 0: HTTP client
		nodes.push({
			id: 'http-client',
			type: 'infra',
			position: { x: COL[0], y: ROW2 },
			data: { label: 'Browser', sublabel: 'HTTP', port: 80, icon: 'globe' },
			draggable: true,
		});

		// Col 1: SNI Router
		nodes.push({
			id: 'sni-router',
			type: 'infra',
			position: { x: COL[1], y: ROW1 },
			data: {
				label: 'SNI Router',
				sublabel: 'TLS routing',
				port: topology.sniRouter.port,
				externalPort: topology.sniRouter.listenPort,
				icon: 'shield',
			},
			draggable: true,
		});

		// Col 1: HTTP Redirect
		nodes.push({
			id: 'http-redirect',
			type: 'infra',
			position: { x: COL[1], y: ROW2 },
			data: {
				label: 'HTTP Redirect',
				sublabel: '301 -> HTTPS',
				port: topology.httpRedirect.port,
				externalPort: topology.httpRedirect.redirectPort,
				icon: 'redirect',
			},
			draggable: true,
		});

		// Col 2: HTTPS Server
		nodes.push({
			id: 'bun-https',
			type: 'infra',
			position: { x: COL[2], y: ROW1 },
			data: {
				label: 'HTTPS Server',
				sublabel: 'Reverse proxy',
				port: topology.httpsServer.port,
				icon: 'server',
			},
			draggable: true,
		});

		// Col 2: Traefik
		nodes.push({
			id: 'traefik',
			type: 'traefik',
			position: { x: COL[2], y: ROW2 },
			data: {
				ip: topology.traefik.ip,
				port: topology.traefik.port,
				domains: topology.traefik.domains,
			},
			draggable: true,
		});

		// Col 3: Service nodes (grouped by unique hostname)
		const uniqueRoutes = new Map<string, (typeof topology.routes)[0]>();
		for (const r of topology.routes) {
			if (!uniqueRoutes.has(r.hostname)) {
				uniqueRoutes.set(r.hostname, r);
			}
		}

		let serviceIdx = 0;
		for (const [hostname, route] of uniqueRoutes) {
			const nodeId = `svc-${hostname}`;
			const hostStat = stats?.hostStats[hostname];

			nodes.push({
				id: nodeId,
				type: 'service',
				position: { x: COL[3], y: 10 * scale + serviceIdx * ROW_HEIGHT },
				data: {
					hostname,
					target: route.target,
					source: route.source,
					containerName: route.containerName,
					stats: hostStat,
				},
				draggable: true,
			});

			// Edge from HTTPS Server to service
			const edgeKey = `${hostname}->${route.target}`;
			const eStat = stats?.edgeStats[edgeKey];

			edges.push({
				id: `e-bun-${hostname}`,
				source: 'bun-https',
				target: nodeId,
				animated: true,
				label: edgeLabel(eStat),
				style: edgeStyle(eStat),
				markerEnd: { type: MarkerType.ArrowClosed, color: edgeStyle(eStat).stroke },
				labelStyle: { fontSize: labelFontSize, fill: '#94a3b8' },
				labelBgStyle: { fill: labelBg, fillOpacity: 0.7 },
				labelBgPadding: [4, 6] as [number, number],
				labelBgBorderRadius: 4,
			});

			serviceIdx++;
		}

		// Infra edges
		edges.push({
			id: 'e-browser-sni',
			source: 'browser',
			target: 'sni-router',
			animated: true,
			style: { stroke: '#6366f1', strokeWidth: 1.5 },
			markerEnd: { type: MarkerType.ArrowClosed, color: '#6366f1' },
		});

		edges.push({
			id: 'e-sni-bun',
			source: 'sni-router',
			target: 'bun-https',
			animated: true,
			label: '*.lvh.me',
			style: { stroke: '#6366f1', strokeWidth: 1.5 },
			markerEnd: { type: MarkerType.ArrowClosed, color: '#6366f1' },
			labelStyle: { fontSize: labelFontSize, fill: '#94a3b8' },
			labelBgStyle: { fill: labelBg, fillOpacity: 0.7 },
			labelBgPadding: [4, 6] as [number, number],
			labelBgBorderRadius: 4,
		});

		edges.push({
			id: 'e-sni-traefik',
			source: 'sni-router',
			target: 'traefik',
			animated: true,
			label: topology.traefik.domains?.[0] ?? 'passthrough',
			style: { stroke: '#f97316', strokeWidth: 1.5 },
			markerEnd: { type: MarkerType.ArrowClosed, color: '#f97316' },
			labelStyle: { fontSize: labelFontSize, fill: '#94a3b8' },
			labelBgStyle: { fill: labelBg, fillOpacity: 0.7 },
			labelBgPadding: [4, 6] as [number, number],
			labelBgBorderRadius: 4,
		});

		edges.push({
			id: 'e-http-redirect',
			source: 'http-client',
			target: 'http-redirect',
			animated: true,
			style: { stroke: '#6366f1', strokeWidth: 1.5 },
			markerEnd: { type: MarkerType.ArrowClosed, color: '#6366f1' },
		});

		return { nodes, edges };
	}, [topology, stats, isDark, scale]);
}
