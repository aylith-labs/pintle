import { CaretDownIcon, CaretRightIcon } from '@phosphor-icons/react';
import { Handle, type NodeProps, Position } from '@xyflow/react';
import type { RouteStats } from '../../types';
import { formatCount } from '../grouping';
import { HEALTH_DOT, healthOf } from '../health';

export type GroupNodeData = {
	label: string;
	count: number;
	stats?: RouteStats;
	expanded: boolean;
};

function useScale() {
	const raw = getComputedStyle(document.documentElement).getPropertyValue('--ui-scale');
	return Number(raw) || 1;
}

export function GroupNode({ data }: NodeProps) {
	const nodeData = data as unknown as GroupNodeData;
	const stats = nodeData.stats;
	const scale = useScale();
	const Caret = nodeData.expanded ? CaretDownIcon : CaretRightIcon;
	const healthColor = HEALTH_DOT[healthOf(stats)];

	return (
		<div
			className="glass rounded-xl border border-gray-200/60 dark:border-zinc-700/60 bg-white/80 dark:bg-zinc-900/80 shadow-lg cursor-pointer select-none"
			style={{ padding: `${10 * scale}px ${12 * scale}px`, minWidth: 200 * scale }}
		>
			<Handle type="target" position={Position.Left} className="!bg-indigo-500 !w-2 !h-2 !border-0" />
			<div className="flex items-center gap-2">
				<div className={`rounded-full ${healthColor} shrink-0`} style={{ width: 8 * scale, height: 8 * scale }} />
				<div className="min-w-0 flex-1">
					<div className="flex items-center" style={{ gap: 6 * scale }}>
						<Caret size={Math.round(12 * scale)} weight="bold" className="shrink-0 text-gray-500 dark:text-zinc-400" />
						<span className="font-semibold text-gray-900 dark:text-zinc-100 truncate" style={{ fontSize: 12 * scale }}>
							{nodeData.label}
						</span>
						<span
							className="inline-flex items-center rounded font-medium uppercase tracking-wider whitespace-nowrap shrink-0 bg-indigo-500/10 text-indigo-600 dark:text-indigo-400"
							style={{ padding: `${2 * scale}px ${6 * scale}px`, fontSize: 9 * scale }}
						>
							{nodeData.count} services
						</span>
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
			<Handle type="source" position={Position.Right} className="!bg-indigo-500 !w-2 !h-2 !border-0" />
		</div>
	);
}
