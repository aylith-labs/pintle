import { MagnifyingGlassIcon, XIcon } from '@phosphor-icons/react';
import { useEffect, useRef, useState } from 'react';
import { isEditableTarget } from '../utils';
import { type FlowFilters, type MatchMode, SOURCE_OPTIONS, type SourceKind } from './filters';
import { GROUPING_MODES, type GroupingMode } from './grouping';
import { HEALTH_DOT, HEALTH_LABEL, HEALTH_STATES, type HealthState } from './health';

const GROUPING_LABELS: Record<GroupingMode, string> = {
	none: 'None',
	domain: 'Domain',
	source: 'Source',
	prefix: 'Prefix',
};

const SOURCE_LABELS: Record<SourceKind, string> = {
	docker: 'Docker',
	static: 'Static',
	traefik: 'Traefik',
};

const MATCH_MODES: MatchMode[] = ['hide', 'spotlight'];
const MATCH_LABELS: Record<MatchMode, string> = {
	hide: 'Hide',
	spotlight: 'Spotlight',
};

const segmentBase = 'rounded-md px-2 py-1 text-[0.625rem] font-medium transition-colors';
const segmentActive = 'bg-indigo-500/10 text-indigo-600 dark:text-indigo-400';
const segmentIdle = 'text-gray-500 dark:text-zinc-400 hover:bg-gray-100 dark:hover:bg-zinc-800';
const segmentDisabled = 'text-gray-300 dark:text-zinc-600 cursor-not-allowed';

const bubbleBase =
	'absolute -top-2 -right-1.5 min-w-[15px] h-[15px] px-1 rounded-full text-[0.5rem] font-semibold leading-[15px] text-center tabular-nums';
const bubbleActive = 'bg-indigo-500 text-white';
const bubbleIdle = 'bg-gray-200 dark:bg-zinc-700 text-gray-600 dark:text-zinc-300';

function FilterChip({
	label,
	count,
	active,
	onClick,
	dot,
}: {
	label: string;
	count: number;
	active: boolean;
	onClick: () => void;
	dot?: string;
}) {
	const disabled = count === 0;
	return (
		<button
			type="button"
			aria-pressed={active}
			disabled={disabled}
			onClick={onClick}
			className={`relative ${segmentBase} flex items-center gap-1.5 ${
				disabled ? segmentDisabled : active ? segmentActive : segmentIdle
			}`}
		>
			{dot && <span className={`h-2 w-2 rounded-full ${dot} ${disabled ? 'opacity-40' : ''}`} />}
			{label}
			{count > 0 && <span className={`${bubbleBase} ${active ? bubbleActive : bubbleIdle}`}>{count}</span>}
		</button>
	);
}

function Divider() {
	return <span className="h-5 w-px bg-gray-200 dark:bg-zinc-700 shrink-0" />;
}

function Label({ children }: { children: string }) {
	return (
		<span className="text-[0.625rem] uppercase tracking-wider text-gray-400 dark:text-zinc-500 shrink-0">
			{children}
		</span>
	);
}

type DiagramToolbarProps = {
	groupingMode: GroupingMode;
	onGroupingChange: (mode: GroupingMode) => void;
	filters: FlowFilters;
	onSearchChange: (value: string) => void;
	onToggleSource: (source: SourceKind) => void;
	onToggleHealth: (health: HealthState) => void;
	onMatchModeChange: (mode: MatchMode) => void;
	filterActive: boolean;
	onClearFilters: () => void;
	sourceCounts: Record<SourceKind, number>;
	healthCounts: Record<HealthState, number>;
};

export function DiagramToolbar({
	groupingMode,
	onGroupingChange,
	filters,
	onSearchChange,
	onToggleSource,
	onToggleHealth,
	onMatchModeChange,
	filterActive,
	onClearFilters,
	sourceCounts,
	healthCounts,
}: DiagramToolbarProps) {
	const searchRef = useRef<HTMLInputElement>(null);
	const [searchFocused, setSearchFocused] = useState(false);

	// `f` focuses the search input — unless the user is already typing in a field.
	useEffect(() => {
		function onKeyDown(event: KeyboardEvent) {
			if (event.key !== 'f' || event.ctrlKey || event.metaKey || event.altKey) return;
			if (isEditableTarget(document.activeElement)) return;
			event.preventDefault();
			searchRef.current?.focus();
		}
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	}, []);

	return (
		<div className="flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-gray-200/60 dark:border-zinc-800 bg-white/60 dark:bg-zinc-900/40 px-3 py-2">
			<div className="flex items-center gap-1.5">
				<Label>Group by</Label>
				<div className="flex items-center gap-0.5">
					{GROUPING_MODES.map((mode) => (
						<button
							key={mode}
							type="button"
							onClick={() => onGroupingChange(mode)}
							className={`${segmentBase} ${mode === groupingMode ? segmentActive : segmentIdle}`}
						>
							{GROUPING_LABELS[mode]}
						</button>
					))}
				</div>
			</div>

			<Divider />

			<div className="relative flex items-center">
				<MagnifyingGlassIcon
					size={13}
					weight="bold"
					className="absolute left-2 text-gray-400 dark:text-zinc-500 pointer-events-none"
				/>
				<input
					ref={searchRef}
					type="text"
					value={filters.search}
					onChange={(event) => onSearchChange(event.target.value)}
					onFocus={() => setSearchFocused(true)}
					onBlur={() => setSearchFocused(false)}
					placeholder="Search hosts"
					className="w-40 rounded-md bg-gray-50 dark:bg-zinc-800 border border-gray-200/60 dark:border-zinc-700 pl-7 pr-7 py-1 text-xs text-gray-900 dark:text-zinc-100 outline-none focus:ring-1 focus:ring-indigo-500/50 placeholder:text-gray-400 dark:placeholder:text-zinc-500"
				/>
				{filters.search && (
					<button
						type="button"
						onClick={() => onSearchChange('')}
						aria-label="Clear search"
						className="absolute right-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-zinc-300"
					>
						<XIcon size={12} weight="bold" />
					</button>
				)}
				{!filters.search && !searchFocused && (
					<kbd className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded border border-gray-200 dark:border-zinc-600 bg-gray-100 dark:bg-zinc-700/70 px-1.5 py-px text-[0.625rem] font-medium leading-none text-gray-400 dark:text-zinc-400 pointer-events-none select-none">
						f
					</kbd>
				)}
			</div>

			<div className="flex items-center gap-1.5">
				<Label>Source</Label>
				<div className="flex items-center gap-1.5">
					{SOURCE_OPTIONS.map((source) => (
						<FilterChip
							key={source}
							label={SOURCE_LABELS[source]}
							count={sourceCounts[source]}
							active={filters.source === source}
							onClick={() => onToggleSource(source)}
						/>
					))}
				</div>
			</div>

			<div className="flex items-center gap-1.5">
				<Label>Health</Label>
				<div className="flex items-center gap-1.5">
					{HEALTH_STATES.map((health) => (
						<FilterChip
							key={health}
							label={HEALTH_LABEL[health]}
							count={healthCounts[health]}
							active={filters.health === health}
							onClick={() => onToggleHealth(health)}
							dot={HEALTH_DOT[health]}
						/>
					))}
				</div>
			</div>

			<Divider />

			<div className="flex items-center gap-1.5">
				<Label>Non-matches</Label>
				<div className="flex items-center gap-0.5">
					{MATCH_MODES.map((mode) => (
						<button
							key={mode}
							type="button"
							onClick={() => onMatchModeChange(mode)}
							className={`${segmentBase} ${mode === filters.matchMode ? segmentActive : segmentIdle}`}
						>
							{MATCH_LABELS[mode]}
						</button>
					))}
				</div>
			</div>

			{filterActive && (
				<button
					type="button"
					onClick={onClearFilters}
					className="flex items-center gap-1 rounded-md px-2 py-1 text-[0.625rem] font-medium text-gray-500 dark:text-zinc-400 hover:bg-gray-100 dark:hover:bg-zinc-800"
				>
					<XIcon size={11} weight="bold" />
					Clear
				</button>
			)}
		</div>
	);
}
