import { ArrowsLeftRight, Cube, File } from '@phosphor-icons/react';
import { Handle, type NodeProps, Position } from '@xyflow/react';
import type { RouteStats } from '../../types';

export type ServiceNodeData = {
	hostname: string;
	target: string;
	source: 'docker' | 'static' | 'traefik';
	containerName?: string;
	stats?: RouteStats;
};

function formatCount(n: number): string {
	if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
	return String(n);
}

const sourceConfig = {
	docker: { color: 'bg-sky-500/10 text-sky-600 dark:text-sky-400', icon: Cube },
	static: { color: 'bg-amber-500/10 text-amber-600 dark:text-amber-400', icon: File },
	traefik: { color: 'bg-orange-500/10 text-orange-600 dark:text-orange-400', icon: ArrowsLeftRight },
};

function useScale() {
	const raw = getComputedStyle(document.documentElement).getPropertyValue('--ui-scale');
	return Number(raw) || 1;
}

export function ServiceNode({ data }: NodeProps) {
	const nodeData = data as unknown as ServiceNodeData;
	const { color: badgeColor, icon: BadgeIcon } = sourceConfig[nodeData.source] ?? sourceConfig.docker;
	const stats = nodeData.stats;
	const errorRate = stats ? stats.errorCount / Math.max(stats.totalRequests, 1) : 0;
	const scale = useScale();

	let healthColor = 'bg-emerald-500';
	if (!stats) healthColor = 'bg-gray-400 dark:bg-zinc-600';
	else if (errorRate > 0.5) healthColor = 'bg-red-500';
	else if (errorRate > 0.1) healthColor = 'bg-amber-500';

	return (
		<div
			className="glass rounded-xl border border-gray-200/60 dark:border-zinc-700/60 bg-white/80 dark:bg-zinc-900/80 shadow-lg"
			style={{ padding: `${10 * scale}px ${12 * scale}px`, minWidth: 200 * scale }}
		>
			<Handle type="target" position={Position.Left} className="!bg-indigo-500 !w-2 !h-2 !border-0" />
			<div className="flex items-center gap-2">
				<div
					className={`rounded-full ${healthColor} shrink-0`}
					style={{ width: 8 * scale, height: 8 * scale }}
				/>
				<div className="min-w-0 flex-1">
					<div className="flex items-center" style={{ gap: 6 * scale }}>
						<a
							href={`https://${nodeData.hostname}`}
							target="_blank"
							rel="noopener noreferrer"
							className="nopan nodrag font-semibold text-gray-900 dark:text-zinc-100 truncate hover:text-indigo-500 dark:hover:text-indigo-400 transition-colors"
							style={{ fontSize: 12 * scale }}
						>
							{nodeData.hostname}
						</a>
						<span
							className={`inline-flex items-center gap-0.5 rounded font-medium uppercase tracking-wider ${badgeColor}`}
							style={{ padding: `${2 * scale}px ${6 * scale}px`, fontSize: 9 * scale }}
						>
							<BadgeIcon size={Math.round(10 * scale)} weight="bold" />
							{nodeData.source}
						</span>
					</div>
					<div className="font-mono text-gray-400 dark:text-zinc-500 truncate" style={{ fontSize: 10 * scale }}>
						{nodeData.target}
					</div>
					{stats && (
						<div
							className="flex items-center text-gray-500 dark:text-zinc-400"
							style={{ gap: 8 * scale, marginTop: 4 * scale, fontSize: 10 * scale }}
						>
							<span>{formatCount(stats.totalRequests)} req</span>
							<span>{stats.avgDurationMs.toFixed(0)}ms avg</span>
							{stats.errorCount > 0 && <span className="text-red-500">{stats.errorCount} err</span>}
						</div>
					)}
				</div>
			</div>
		</div>
	);
}
