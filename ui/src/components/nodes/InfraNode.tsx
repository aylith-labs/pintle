import { ArrowsLeftRight, Globe, HardDrive, ShieldCheck } from "@phosphor-icons/react";
import { Handle, type NodeProps, Position } from "@xyflow/react";

export type InfraNodeData = {
	label: string;
	sublabel?: string;
	port: number;
	externalPort?: number;
	icon: "globe" | "shield" | "server" | "redirect";
};

const icons = {
	globe: Globe,
	shield: ShieldCheck,
	server: HardDrive,
	redirect: ArrowsLeftRight,
};

export function InfraNode({ data }: NodeProps) {
	const nodeData = data as unknown as InfraNodeData;
	const Icon = icons[nodeData.icon] ?? Globe;

	return (
		<div className="glass rounded-lg border border-gray-200/60 dark:border-zinc-700/60 bg-white/80 dark:bg-zinc-900/80 px-2.5 py-1.5 shadow-lg min-w-[140px]">
			<Handle type="target" position={Position.Left} className="!bg-indigo-500 !w-2 !h-2 !border-0" />
			<div className="flex items-center gap-2.5">
				<div className="flex items-center justify-center w-8 h-8 rounded-lg bg-indigo-500/10 dark:bg-indigo-500/20 text-indigo-500">
					<Icon size={18} weight="bold" />
				</div>
				<div>
					<div className="text-xs font-semibold text-gray-900 dark:text-zinc-100">{nodeData.label}</div>
					{nodeData.sublabel && <div className="text-[10px] text-gray-500 dark:text-zinc-400">{nodeData.sublabel}</div>}
					<div className="text-[10px] font-mono text-gray-400 dark:text-zinc-500">
						:{nodeData.port}
						{nodeData.externalPort != null && <span className="text-indigo-400"> (:{nodeData.externalPort})</span>}
					</div>
				</div>
			</div>
			<Handle type="source" position={Position.Right} className="!bg-indigo-500 !w-2 !h-2 !border-0" />
		</div>
	);
}
