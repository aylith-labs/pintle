import { useVirtualizer } from '@tanstack/react-virtual';
import { useEffect, useMemo, useRef, useState } from 'react';
import { FilterBar } from '../components/FilterBar';
import { StatsBar } from '../components/StatsBar';
import { useActivityRequests } from '../hooks';
import type { ActivityFilters, DurationFilter, ProxyRequest, ProxyStats, ProxyTopology, TimeRange } from '../types';
import { formatTime, formatTimeAgo, methodColor, statusColor } from '../utils';

type TimeFormat = 'absolute' | 'relative';

type SortKey = 'time' | 'method' | 'hostname' | 'path' | 'status' | 'duration';
type SortDir = 'asc' | 'desc';
type SortState = { key: SortKey; dir: SortDir };

function nextSort(current: SortState, key: SortKey): SortState {
	if (current.key !== key) {
		return { key, dir: key === 'time' ? 'desc' : 'asc' };
	}
	return { key, dir: current.dir === 'asc' ? 'desc' : 'asc' };
}

function compareRequests(a: ProxyRequest, b: ProxyRequest, sort: SortState): number {
	const dir = sort.dir === 'asc' ? 1 : -1;
	switch (sort.key) {
		case 'time':
			return (a.timestamp - b.timestamp) * dir;
		case 'duration':
			return (a.durationMs - b.durationMs) * dir;
		case 'status':
			return (a.status - b.status) * dir;
		case 'method':
			return a.method.localeCompare(b.method) * dir;
		case 'hostname':
			return a.hostname.localeCompare(b.hostname) * dir;
		case 'path':
			return a.path.localeCompare(b.path) * dir;
	}
}

type ActivityPageProps = {
	topology: ProxyTopology | null;
	stats: ProxyStats | null;
};

const defaultFilters: ActivityFilters = {
	timeRange: 'all',
	method: 'ALL',
	host: 'ALL',
	status: 'ALL',
	duration: 'ALL',
};

function timeRangeToMs(range: Exclude<TimeRange, 'all'>): number {
	const map: Record<Exclude<TimeRange, 'all'>, number> = {
		'5m': 5 * 60_000,
		'15m': 15 * 60_000,
		'30m': 30 * 60_000,
		'1h': 60 * 60_000,
		'6h': 6 * 60 * 60_000,
		'1d': 24 * 60 * 60_000,
		'1w': 7 * 24 * 60 * 60_000,
	};
	return map[range];
}

function matchesDuration(ms: number, filter: DurationFilter): boolean {
	switch (filter) {
		case '<100ms':
			return ms < 100;
		case '<500ms':
			return ms < 500;
		case '<1s':
			return ms < 1000;
		case '>1s':
			return ms >= 1000;
		default:
			return true;
	}
}

function useTick(enabled: boolean, intervalMs: number): number {
	const [now, setNow] = useState(() => Date.now());
	useEffect(() => {
		if (!enabled) return;
		const id = setInterval(() => setNow(Date.now()), intervalMs);
		return () => clearInterval(id);
	}, [enabled, intervalMs]);
	return now;
}

function SortHeader({
	label,
	columnKey,
	sort,
	onSort,
	align = 'left',
}: {
	label: string;
	columnKey: SortKey;
	sort: SortState;
	onSort: (key: SortKey) => void;
	align?: 'left' | 'right';
}) {
	const isActive = sort.key === columnKey;
	const arrow = isActive ? (sort.dir === 'asc' ? '↑' : '↓') : '';
	return (
		<button
			type="button"
			onClick={() => onSort(columnKey)}
			className={`px-3 py-2 font-medium transition-colors flex items-center gap-1 ${
				align === 'right' ? 'justify-end' : ''
			} ${
				isActive
					? 'text-indigo-600 dark:text-indigo-400'
					: 'text-gray-500 dark:text-zinc-400 hover:text-gray-800 dark:hover:text-zinc-200'
			}`}
		>
			<span>{label}</span>
			<span className="w-2 text-[0.6rem]">{arrow}</span>
		</button>
	);
}

function VirtualRequestTable({
	requests,
	timeFormat,
	onTimeFormatChange,
	sort,
	onSortChange,
}: {
	requests: ProxyRequest[];
	timeFormat: TimeFormat;
	onTimeFormatChange: (format: TimeFormat) => void;
	sort: SortState;
	onSortChange: (key: SortKey) => void;
}) {
	const parentRef = useRef<HTMLDivElement>(null);
	const now = useTick(timeFormat === 'relative', 1000);

	const rowVirtualizer = useVirtualizer({
		count: requests.length,
		getScrollElement: () => parentRef.current,
		estimateSize: () => 32,
		overscan: 20,
	});

	const gridCols = timeFormat === 'relative' ? '100px_60px_1fr_1fr_60px_70px' : '110px_60px_1fr_1fr_60px_70px';

	const header = (
		<div className="px-4 py-2 border-b border-gray-200/60 dark:border-zinc-800 flex items-center justify-between">
			<div className="flex items-center gap-3">
				<span className="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-zinc-400">
					Requests
				</span>
				<TimeFormatToggle value={timeFormat} onChange={onTimeFormatChange} />
			</div>
			<span className="text-[0.625rem] font-mono text-gray-400 dark:text-zinc-500">{requests.length} results</span>
		</div>
	);

	if (requests.length === 0) {
		return (
			<div className="flex-1 glass rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 shadow-sm overflow-hidden flex flex-col">
				{header}
				<div className="flex-1 flex items-center justify-center text-sm text-gray-400 dark:text-zinc-500">
					No requests match the current filters.
				</div>
			</div>
		);
	}

	return (
		<div className="flex-1 min-h-0 glass rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 shadow-sm overflow-hidden flex flex-col">
			{header}

			<div
				className="bg-gray-50 dark:bg-zinc-900 border-b border-gray-200/60 dark:border-zinc-800 text-xs font-mono grid"
				style={{ gridTemplateColumns: gridCols.replaceAll('_', ' ') }}
			>
				<SortHeader label="Time" columnKey="time" sort={sort} onSort={onSortChange} />
				<SortHeader label="Method" columnKey="method" sort={sort} onSort={onSortChange} />
				<SortHeader label="Host" columnKey="hostname" sort={sort} onSort={onSortChange} />
				<SortHeader label="Path" columnKey="path" sort={sort} onSort={onSortChange} />
				<SortHeader label="Status" columnKey="status" sort={sort} onSort={onSortChange} align="right" />
				<SortHeader label="Duration" columnKey="duration" sort={sort} onSort={onSortChange} align="right" />
			</div>

			<div ref={parentRef} className="flex-1 overflow-y-auto">
				<div style={{ height: `${rowVirtualizer.getTotalSize()}px`, position: 'relative' }}>
					{rowVirtualizer.getVirtualItems().map((virtualRow) => {
						const req = requests[virtualRow.index];
						const timeLabel = timeFormat === 'relative' ? formatTimeAgo(req.timestamp, now) : formatTime(req.timestamp);
						return (
							<div
								key={virtualRow.key}
								data-index={virtualRow.index}
								className="border-b border-gray-100/60 dark:border-zinc-800/60 hover:bg-gray-50 dark:hover:bg-zinc-800/50 text-xs font-mono grid"
								style={{
									position: 'absolute',
									top: 0,
									left: 0,
									width: '100%',
									height: `${virtualRow.size}px`,
									transform: `translateY(${virtualRow.start}px)`,
									gridTemplateColumns: gridCols.replaceAll('_', ' '),
								}}
							>
								<div
									className="px-3 py-1.5 text-gray-400 dark:text-zinc-500 truncate"
									title={new Date(req.timestamp).toLocaleString()}
								>
									{timeLabel}
								</div>
								<div className={`px-3 py-1.5 font-medium ${methodColor(req.method)}`}>{req.method}</div>
								<div className="px-3 py-1.5 text-gray-700 dark:text-zinc-300 truncate">{req.hostname}</div>
								<div className="px-3 py-1.5 text-gray-500 dark:text-zinc-400 truncate">{req.path}</div>
								<div className={`px-3 py-1.5 text-right font-medium ${statusColor(req.status)}`}>{req.status}</div>
								<div className="px-3 py-1.5 text-right text-gray-400 dark:text-zinc-500">
									{req.durationMs.toFixed(0)}ms
								</div>
							</div>
						);
					})}
				</div>
			</div>
		</div>
	);
}

function TimeFormatToggle({ value, onChange }: { value: TimeFormat; onChange: (value: TimeFormat) => void }) {
	return (
		<div className="inline-flex rounded border border-gray-200/60 dark:border-zinc-700 bg-gray-50 dark:bg-zinc-800 text-[0.625rem] font-mono uppercase tracking-wider">
			{(['absolute', 'relative'] as const).map((opt) => (
				<button
					key={opt}
					type="button"
					onClick={() => onChange(opt)}
					className={`px-2 py-0.5 transition-colors ${
						value === opt
							? 'bg-indigo-500 text-white'
							: 'text-gray-500 dark:text-zinc-400 hover:text-gray-800 dark:hover:text-zinc-200'
					}`}
				>
					{opt === 'absolute' ? 'Time' : 'Ago'}
				</button>
			))}
		</div>
	);
}

export function ActivityPage({ topology, stats }: ActivityPageProps) {
	const { data: requests } = useActivityRequests();
	const [filters, setFilters] = useState<ActivityFilters>(defaultFilters);
	const [timeFormat, setTimeFormat] = useState<TimeFormat>('absolute');
	const [sort, setSort] = useState<SortState>({ key: 'time', dir: 'desc' });

	const handleSort = (key: SortKey) => setSort((prev) => nextSort(prev, key));

	const uniqueHosts = useMemo(() => {
		if (!requests) return [];
		return [...new Set(requests.map((r) => r.hostname))].sort();
	}, [requests]);

	const filteredRequests = useMemo(() => {
		if (!requests) return [];
		const filtered = requests.filter((req) => {
			if (filters.timeRange !== 'all') {
				const ms = timeRangeToMs(filters.timeRange);
				if (Date.now() - req.timestamp > ms) return false;
			}
			if (filters.method !== 'ALL' && req.method !== filters.method) return false;
			if (filters.host !== 'ALL' && req.hostname !== filters.host) return false;
			if (filters.status !== 'ALL') {
				const century = Math.floor(req.status / 100);
				const expected = Number.parseInt(filters.status[0], 10);
				if (century !== expected) return false;
			}
			if (filters.duration !== 'ALL') {
				if (!matchesDuration(req.durationMs, filters.duration)) return false;
			}
			return true;
		});
		return [...filtered].sort((a, b) => compareRequests(a, b, sort));
	}, [requests, filters, sort]);

	return (
		<div className="flex flex-col gap-3 h-full">
			<StatsBar topology={topology} stats={stats} />
			<FilterBar filters={filters} onFiltersChange={setFilters} hosts={uniqueHosts} />
			<VirtualRequestTable
				requests={filteredRequests}
				timeFormat={timeFormat}
				onTimeFormatChange={setTimeFormat}
				sort={sort}
				onSortChange={handleSort}
			/>
		</div>
	);
}
