import { Background, Controls, type NodeTypes, ReactFlow } from "@xyflow/react";
import type { ProxyStats, ProxyTopology } from "../types";
import { InfraNode } from "./nodes/InfraNode";
import { ServiceNode } from "./nodes/ServiceNode";
import { TraefikNode } from "./nodes/TraefikNode";
import { useFlowLayout } from "./useFlowLayout";

const nodeTypes: NodeTypes = {
	infra: InfraNode,
	service: ServiceNode,
	traefik: TraefikNode,
};

type FlowDiagramProps = {
	topology: ProxyTopology | null;
	stats: ProxyStats | null;
};

export function FlowDiagram({ topology, stats }: FlowDiagramProps) {
	const { nodes, edges } = useFlowLayout(topology, stats);

	return (
		<div className="w-full h-[500px] rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white dark:bg-zinc-950 overflow-hidden">
			<ReactFlow
				nodes={nodes}
				edges={edges}
				nodeTypes={nodeTypes}
				fitView
				fitViewOptions={{ padding: 0.3 }}
				proOptions={{ hideAttribution: true }}
				nodesDraggable
				nodesConnectable={false}
				elementsSelectable={false}
				minZoom={0.5}
				maxZoom={1.5}
			>
				<Background gap={20} size={1} color="#e5e7eb" className="dark:!text-zinc-800" />
				<Controls
					showInteractive={false}
					className="!bg-white/80 dark:!bg-zinc-900/80 !border-gray-200 dark:!border-zinc-700 !shadow-lg !rounded-lg [&_button]:!bg-transparent [&_button]:!border-gray-200 dark:[&_button]:!border-zinc-700 [&_button]:!text-gray-600 dark:[&_button]:!text-zinc-400"
				/>
			</ReactFlow>
		</div>
	);
}
