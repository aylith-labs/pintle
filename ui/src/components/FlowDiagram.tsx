import { Background, Controls, type Node, type NodeTypes, ReactFlow } from '@xyflow/react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ProxyStats, ProxyTopology } from '../types';
import { DiagramShortcuts } from './DiagramShortcuts';
import { DiagramToolbar } from './DiagramToolbar';
import { type FlowFilters, isFilterActive, type MatchMode, type SourceKind } from './filters';
import { GROUPING_MODES, type GroupingMode } from './grouping';
import type { HealthState } from './health';
import { GroupNode } from './nodes/GroupNode';
import { InfraNode } from './nodes/InfraNode';
import { ServiceNode } from './nodes/ServiceNode';
import { TraefikNode } from './nodes/TraefikNode';
import { useFlowLayout } from './useFlowLayout';

const nodeTypes: NodeTypes = {
	infra: InfraNode,
	service: ServiceNode,
	traefik: TraefikNode,
	serviceGroup: GroupNode,
};

const GROUPING_STORAGE_KEY = 'proxy-flow-grouping';
const MATCH_MODE_STORAGE_KEY = 'proxy-flow-match-mode';

function loadGroupingMode(): GroupingMode {
	const stored = localStorage.getItem(GROUPING_STORAGE_KEY);
	return GROUPING_MODES.includes(stored as GroupingMode) ? (stored as GroupingMode) : 'domain';
}

function loadMatchMode(): MatchMode {
	return localStorage.getItem(MATCH_MODE_STORAGE_KEY) === 'spotlight' ? 'spotlight' : 'hide';
}

type FlowDiagramProps = {
	topology: ProxyTopology | null;
	stats: ProxyStats | null;
	scale: number;
};

export function FlowDiagram({ topology, stats, scale }: FlowDiagramProps) {
	const [groupingMode, setGroupingModeState] = useState<GroupingMode>(loadGroupingMode);
	const [expandedGroups, setExpandedGroups] = useState<ReadonlySet<string>>(new Set());
	const [search, setSearch] = useState('');
	const [source, setSource] = useState<SourceKind | null>(null);
	const [health, setHealth] = useState<HealthState | null>(null);
	const [matchMode, setMatchModeState] = useState<MatchMode>(loadMatchMode);

	const filters: FlowFilters = useMemo(
		() => ({ search, source, health, matchMode }),
		[search, source, health, matchMode],
	);
	const filterActive = isFilterActive(filters);

	const setGroupingMode = useCallback((mode: GroupingMode) => {
		setGroupingModeState(mode);
		localStorage.setItem(GROUPING_STORAGE_KEY, mode);
		// Group keys are mode-specific; collapse everything on switch
		setExpandedGroups(new Set());
	}, []);

	const setMatchMode = useCallback((mode: MatchMode) => {
		setMatchModeState(mode);
		localStorage.setItem(MATCH_MODE_STORAGE_KEY, mode);
	}, []);

	const toggleSource = useCallback((next: SourceKind) => {
		setSource((current) => (current === next ? null : next));
	}, []);

	const toggleHealth = useCallback((next: HealthState) => {
		setHealth((current) => (current === next ? null : next));
	}, []);

	const clearFilters = useCallback(() => {
		setSearch('');
		setSource(null);
		setHealth(null);
	}, []);

	const onNodeClick = useCallback((_event: React.MouseEvent, node: Node) => {
		if (node.type !== 'serviceGroup') return;
		const key = node.id.slice('grp-'.length);
		setExpandedGroups((prev) => {
			const next = new Set(prev);
			if (next.has(key)) next.delete(key);
			else next.add(key);
			return next;
		});
	}, []);

	const { nodes, edges, toolbarActive, sourceCounts, healthCounts } = useFlowLayout(
		topology,
		stats,
		scale,
		groupingMode,
		expandedGroups,
		filters,
	);

	// Drop an active source/health selection once nothing matches it anymore
	// (its chip becomes disabled and can no longer be clicked to clear).
	useEffect(() => {
		if (source && sourceCounts[source] === 0) setSource(null);
	}, [source, sourceCounts]);
	useEffect(() => {
		if (health && healthCounts[health] === 0) setHealth(null);
	}, [health, healthCounts]);

	return (
		<div className="w-full flex-1 min-h-[300px] rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white dark:bg-zinc-950 overflow-hidden flex flex-col">
			{toolbarActive && (
				<DiagramToolbar
					groupingMode={groupingMode}
					onGroupingChange={setGroupingMode}
					filters={filters}
					onSearchChange={setSearch}
					onToggleSource={toggleSource}
					onToggleHealth={toggleHealth}
					onMatchModeChange={setMatchMode}
					filterActive={filterActive}
					onClearFilters={clearFilters}
					sourceCounts={sourceCounts}
					healthCounts={healthCounts}
				/>
			)}
			<div className="flex-1 min-h-0">
				<ReactFlow
					nodes={nodes}
					edges={edges}
					nodeTypes={nodeTypes}
					onNodeClick={onNodeClick}
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
					<DiagramShortcuts />
				</ReactFlow>
			</div>
		</div>
	);
}
