import { Cube, File } from "@phosphor-icons/react";
import { Handle, type NodeProps, Position } from "@xyflow/react";
import type { RouteStats } from "../../types";

export type ServiceNodeData = {
	hostname: string;
	target: string;
	source: "docker" | "static";
	containerName?: string;
	stats?: RouteStats;
};

function formatCount(n: number): string {
	if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
	return String(n);
}

export function ServiceNode({ data }: NodeProps) {
	const nodeData = data as unknown as ServiceNodeData;
	const isDocker = nodeData.source === "docker";
	const stats = nodeData.stats;
	const errorRate = stats ? stats.errorCount / Math.max(stats.totalRequests, 1) : 0;

	let healthColor = "bg-emerald-500";
	if (!stats) healthColor = "bg-gray-400 dark:bg-zinc-600";
	else if (errorRate > 0.5) healthColor = "bg-red-500";
	else if (errorRate > 0.1) healthColor = "bg-amber-500";

	return (
		<div className="glass rounded-xl border border-gray-200/60 dark:border-zinc-700/60 bg-white/80 dark:bg-zinc-900/80 px-3 py-2.5 shadow-lg min-w-[200px]">
			<Handle type="target" position={Position.Left} className="!bg-indigo-500 !w-2 !h-2 !border-0" />
			<div className="flex items-center gap-2">
				<div className={`w-2 h-2 rounded-full ${healthColor} shrink-0`} />
				<div className="min-w-0 flex-1">
					<div className="flex items-center gap-1.5">
						<span className="text-xs font-semibold text-gray-900 dark:text-zinc-100 truncate">{nodeData.hostname}</span>
						<span
							className={`inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[9px] font-medium uppercase tracking-wider ${
								isDocker
									? "bg-sky-500/10 text-sky-600 dark:text-sky-400"
									: "bg-amber-500/10 text-amber-600 dark:text-amber-400"
							}`}
						>
							{isDocker ? <Cube size={10} weight="bold" /> : <File size={10} weight="bold" />}
							{nodeData.source}
						</span>
					</div>
					<div className="text-[10px] font-mono text-gray-400 dark:text-zinc-500 truncate">{nodeData.target}</div>
					{stats && (
						<div className="flex items-center gap-2 mt-1 text-[10px] text-gray-500 dark:text-zinc-400">
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
