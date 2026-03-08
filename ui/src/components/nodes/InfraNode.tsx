import { ArrowsLeftRight, Globe, HardDrive, ShieldCheck } from '@phosphor-icons/react';
import { Handle, type NodeProps, Position } from '@xyflow/react';

export type InfraNodeData = {
	label: string;
	sublabel?: string;
	port: number;
	externalPort?: number;
	icon: 'globe' | 'shield' | 'server' | 'redirect';
};

const icons = {
	globe: Globe,
	shield: ShieldCheck,
	server: HardDrive,
	redirect: ArrowsLeftRight,
};

function useScale() {
	const raw = getComputedStyle(document.documentElement).getPropertyValue('--ui-scale');
	return Number(raw) || 1;
}

export function InfraNode({ data }: NodeProps) {
	const nodeData = data as unknown as InfraNodeData;
	const Icon = icons[nodeData.icon] ?? Globe;
	const scale = useScale();
	const iconSize = Math.round(18 * scale);
	const iconBox = Math.round(32 * scale);

	return (
		<div
			className="glass rounded-lg border border-gray-200/60 dark:border-zinc-700/60 bg-white/80 dark:bg-zinc-900/80 shadow-lg"
			style={{ padding: `${6 * scale}px ${10 * scale}px`, minWidth: 140 * scale }}
		>
			<Handle type="target" position={Position.Left} className="!bg-indigo-500 !w-2 !h-2 !border-0" />
			<div className="flex items-center" style={{ gap: 10 * scale }}>
				<div
					className="flex items-center justify-center rounded-lg bg-indigo-500/10 dark:bg-indigo-500/20 text-indigo-500"
					style={{ width: iconBox, height: iconBox }}
				>
					<Icon size={iconSize} weight="bold" />
				</div>
				<div>
					<div className="font-semibold text-gray-900 dark:text-zinc-100" style={{ fontSize: 12 * scale }}>
						{nodeData.label}
					</div>
					{nodeData.sublabel && (
						<div className="text-gray-500 dark:text-zinc-400" style={{ fontSize: 10 * scale }}>
							{nodeData.sublabel}
						</div>
					)}
					<div className="font-mono text-gray-400 dark:text-zinc-500" style={{ fontSize: 10 * scale }}>
						:{nodeData.port}
						{nodeData.externalPort != null && <span className="text-indigo-400"> (:{nodeData.externalPort})</span>}
					</div>
				</div>
			</div>
			<Handle type="source" position={Position.Right} className="!bg-indigo-500 !w-2 !h-2 !border-0" />
		</div>
	);
}
