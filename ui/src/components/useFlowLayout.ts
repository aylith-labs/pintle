import { type Edge, MarkerType, type Node } from '@xyflow/react';
import { useMemo } from 'react';
import type { ProxyRoute, ProxyStats, ProxyTopology, RouteStats } from '../types';
import { EMPTY_FILTERS, type FlowFilters, isFilterActive, matchesRoute, type SourceKind } from './filters';
import { aggregateStats, GROUPING_THRESHOLD, type GroupingMode, groupRoutes } from './grouping';
import { type HealthState, healthOf } from './health';

export type SourceCounts = Record<SourceKind, number>;
export type HealthCounts = Record<HealthState, number>;

const emptySourceCounts = (): SourceCounts => ({ docker: 0, static: 0, traefik: 0 });
const emptyHealthCounts = (): HealthCounts => ({ healthy: 0, warning: 0, error: 0, idle: 0 });

const BASE_COL = [0, 220, 460, 720, 980];
const BASE_ROW_HEIGHT = 90;

const EMPTY_SET: ReadonlySet<string> = new Set();

const DIM_NODE = 0.2;
const DIM_EDGE = 0.12;

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

export function useFlowLayout(
	topology: ProxyTopology | null,
	stats: ProxyStats | null,
	scale = 1,
	groupingMode: GroupingMode = 'domain',
	expandedGroups: ReadonlySet<string> = EMPTY_SET,
	filters: FlowFilters = EMPTY_FILTERS,
) {
	const isDark = document.documentElement.classList.contains('dark');

	return useMemo(() => {
		if (!topology)
			return {
				nodes: [] as Node[],
				edges: [] as Edge[],
				toolbarActive: false,
				sourceCounts: emptySourceCounts(),
				healthCounts: emptyHealthCounts(),
			};

		const COL = BASE_COL.map((c) => c * scale);
		const ROW_HEIGHT = BASE_ROW_HEIGHT * scale;
		const labelBg = isDark ? '#09090b' : '#fafafa';

		const nodes: Node[] = [];
		const edges: Edge[] = [];
		const ROW1 = 60 * scale;
		const ROW2 = 200 * scale;
		const labelFontSize = 10 * scale;

		const edgeLabelProps = {
			labelStyle: { fontSize: labelFontSize, fill: '#94a3b8' },
			labelBgStyle: { fill: labelBg, fillOpacity: 0.7 },
			labelBgPadding: [4, 6] as [number, number],
			labelBgBorderRadius: 4,
		};

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

		// Service nodes (one per unique hostname), grouped/filtered when there are many
		const uniqueRoutes = new Map<string, ProxyRoute>();
		for (const r of topology.routes) {
			if (!uniqueRoutes.has(r.hostname)) {
				uniqueRoutes.set(r.hostname, r);
			}
		}

		function pushServiceNode(route: ProxyRoute, x: number, y: number, sourceNodeId: string, dimmed: boolean) {
			const nodeId = `svc-${route.hostname}`;
			const hostStat = stats?.hostStats[route.hostname];

			nodes.push({
				id: nodeId,
				type: 'service',
				position: { x, y },
				data: {
					hostname: route.hostname,
					target: route.target,
					source: route.source,
					containerName: route.containerName,
					stats: hostStat,
				},
				draggable: true,
				style: dimmed ? { opacity: DIM_NODE } : undefined,
			});

			const edgeKey = `${route.hostname}->${route.target}`;
			const eStat = stats?.edgeStats[edgeKey];
			const baseStyle = edgeStyle(eStat);

			edges.push({
				id: `e-${sourceNodeId}-${route.hostname}`,
				source: sourceNodeId,
				target: nodeId,
				animated: true,
				label: edgeLabel(eStat),
				style: { ...baseStyle, opacity: dimmed ? DIM_EDGE : 1 },
				markerEnd: { type: MarkerType.ArrowClosed, color: baseStyle.stroke },
				...edgeLabelProps,
			});
		}

		// Per-chip counts over the full unique-route set (independent of active filters).
		const sourceCounts = emptySourceCounts();
		const healthCounts = emptyHealthCounts();
		for (const route of uniqueRoutes.values()) {
			sourceCounts[route.source]++;
			healthCounts[healthOf(stats?.hostStats[route.hostname])]++;
		}

		// Toolbar (grouping + filtering) only appears once the fan-out is large.
		const toolbarActive = uniqueRoutes.size > GROUPING_THRESHOLD;
		const effectiveMode: GroupingMode = toolbarActive ? groupingMode : 'none';

		const filtering = toolbarActive && isFilterActive(filters);
		const spotlight = filters.matchMode === 'spotlight';
		const hideMode = filtering && !spotlight;

		const matches = (route: ProxyRoute) => matchesRoute(route, stats, filters);
		const dimmedFor = (route: ProxyRoute) => filtering && spotlight && !matches(route);

		const allRoutes = [...uniqueRoutes.values()];
		const visibleRoutes = hideMode ? allRoutes.filter(matches) : allRoutes;

		if (effectiveMode === 'none') {
			visibleRoutes.forEach((route, idx) => {
				pushServiceNode(route, COL[3], 10 * scale + idx * ROW_HEIGHT, 'bun-https', dimmedFor(route));
			});
		} else {
			const groups = groupRoutes(visibleRoutes, effectiveMode);
			let cursorY = 10 * scale;

			for (const group of groups) {
				if (group.routes.length === 1) {
					const route = group.routes[0];
					pushServiceNode(route, COL[3], cursorY, 'bun-https', dimmedFor(route));
					cursorY += ROW_HEIGHT;
					continue;
				}

				const expanded = expandedGroups.has(group.key);
				const groupId = `grp-${group.key}`;
				const agg = aggregateStats(
					group.routes.map((route) => route.hostname),
					stats?.hostStats,
				);
				const groupDimmed = filtering && spotlight && !group.routes.some(matches);
				const baseStyle = edgeStyle(agg);

				nodes.push({
					id: groupId,
					type: 'serviceGroup',
					position: { x: COL[3], y: cursorY },
					data: { label: group.label, count: group.routes.length, stats: agg, expanded },
					draggable: true,
					style: groupDimmed ? { opacity: DIM_NODE } : undefined,
				});

				edges.push({
					id: `e-bun-${groupId}`,
					source: 'bun-https',
					target: groupId,
					animated: true,
					label: edgeLabel(agg),
					style: { ...baseStyle, opacity: groupDimmed ? DIM_EDGE : 1 },
					markerEnd: { type: MarkerType.ArrowClosed, color: baseStyle.stroke },
					...edgeLabelProps,
				});

				if (expanded) {
					group.routes.forEach((route, memberIdx) => {
						pushServiceNode(route, COL[4], cursorY + memberIdx * ROW_HEIGHT, groupId, dimmedFor(route));
					});
					cursorY += group.routes.length * ROW_HEIGHT;
				} else {
					cursorY += ROW_HEIGHT;
				}
			}
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
			...edgeLabelProps,
		});

		edges.push({
			id: 'e-sni-traefik',
			source: 'sni-router',
			target: 'traefik',
			animated: true,
			label: topology.traefik.domains?.[0] ?? 'passthrough',
			style: { stroke: '#f97316', strokeWidth: 1.5 },
			markerEnd: { type: MarkerType.ArrowClosed, color: '#f97316' },
			...edgeLabelProps,
		});

		edges.push({
			id: 'e-http-redirect',
			source: 'http-client',
			target: 'http-redirect',
			animated: true,
			style: { stroke: '#6366f1', strokeWidth: 1.5 },
			markerEnd: { type: MarkerType.ArrowClosed, color: '#6366f1' },
		});

		return { nodes, edges, toolbarActive, sourceCounts, healthCounts };
	}, [topology, stats, isDark, scale, groupingMode, expandedGroups, filters]);
}
