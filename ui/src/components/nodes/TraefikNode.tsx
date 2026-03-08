import { ArrowsLeftRight } from '@phosphor-icons/react';
import { Handle, type NodeProps, Position } from '@xyflow/react';

export type TraefikNodeData = {
	ip: string | null;
	port: number;
	domains: string[];
};

function useScale() {
	const raw = getComputedStyle(document.documentElement).getPropertyValue('--ui-scale');
	return Number(raw) || 1;
}

export function TraefikNode({ data }: NodeProps) {
	const nodeData = data as unknown as TraefikNodeData;
	const isRunning = nodeData.ip != null;
	const scale = useScale();
	const iconSize = Math.round(18 * scale);
	const iconBox = Math.round(32 * scale);

	return (
		<div
			className={`glass rounded-lg border shadow-lg ${
				isRunning
					? 'border-orange-300/60 dark:border-orange-700/60 bg-orange-50/80 dark:bg-orange-950/40'
					: 'border-red-300/60 dark:border-red-800/60 bg-red-50/80 dark:bg-red-950/40'
			}`}
			style={{ padding: `${6 * scale}px ${10 * scale}px`, minWidth: 140 * scale }}
		>
			<Handle type="target" position={Position.Left} className="!bg-orange-500 !w-2 !h-2 !border-0" />
			<div className="flex items-center" style={{ gap: 10 * scale }}>
				<div
					className={`flex items-center justify-center rounded-lg ${
						isRunning
							? 'bg-orange-500/10 dark:bg-orange-500/20 text-orange-500'
							: 'bg-red-500/10 dark:bg-red-500/20 text-red-500'
					}`}
					style={{ width: iconBox, height: iconBox }}
				>
					<ArrowsLeftRight size={iconSize} weight="bold" />
				</div>
				<div>
					<div className="font-semibold text-gray-900 dark:text-zinc-100" style={{ fontSize: 12 * scale }}>
						Traefik
					</div>
					<div className="font-mono text-gray-500 dark:text-zinc-400" style={{ fontSize: 10 * scale }}>
						{isRunning ? `${nodeData.ip}:${nodeData.port}` : 'Not Running'}
					</div>
					<div className="text-gray-400 dark:text-zinc-500" style={{ fontSize: 10 * scale }}>
						TCP passthrough
					</div>
				</div>
			</div>
			<Handle type="source" position={Position.Right} className="!bg-orange-500 !w-2 !h-2 !border-0" />
		</div>
	);
}
