import { ArrowsLeftRight } from "@phosphor-icons/react";
import { Handle, type NodeProps, Position } from "@xyflow/react";

export type TraefikNodeData = {
	ip: string | null;
	port: number;
	domains: string[];
};

export function TraefikNode({ data }: NodeProps) {
	const nodeData = data as unknown as TraefikNodeData;
	const isRunning = nodeData.ip != null;

	return (
		<div
			className={`glass rounded-lg border px-2.5 py-1.5 shadow-lg min-w-[140px] ${
				isRunning
					? "border-orange-300/60 dark:border-orange-700/60 bg-orange-50/80 dark:bg-orange-950/40"
					: "border-red-300/60 dark:border-red-800/60 bg-red-50/80 dark:bg-red-950/40"
			}`}
		>
			<Handle type="target" position={Position.Left} className="!bg-orange-500 !w-2 !h-2 !border-0" />
			<div className="flex items-center gap-2.5">
				<div
					className={`flex items-center justify-center w-8 h-8 rounded-lg ${
						isRunning
							? "bg-orange-500/10 dark:bg-orange-500/20 text-orange-500"
							: "bg-red-500/10 dark:bg-red-500/20 text-red-500"
					}`}
				>
					<ArrowsLeftRight size={18} weight="bold" />
				</div>
				<div>
					<div className="text-xs font-semibold text-gray-900 dark:text-zinc-100">Traefik</div>
					<div className="text-[10px] font-mono text-gray-500 dark:text-zinc-400">
						{isRunning ? `${nodeData.ip}:${nodeData.port}` : "Not Running"}
					</div>
					<div className="text-[10px] text-gray-400 dark:text-zinc-500">TCP passthrough</div>
				</div>
			</div>
			<Handle type="source" position={Position.Right} className="!bg-orange-500 !w-2 !h-2 !border-0" />
		</div>
	);
}
