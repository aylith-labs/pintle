import { useVirtualizer } from "@tanstack/react-virtual";
import { useMemo, useRef, useState } from "react";
import { FilterBar } from "../components/FilterBar";
import { StatsBar } from "../components/StatsBar";
import { useActivityRequests } from "../hooks";
import type { ActivityFilters, DurationFilter, ProxyRequest, ProxyStats, ProxyTopology, TimeRange } from "../types";
import { formatTime, methodColor, statusColor } from "../utils";

type ActivityPageProps = {
	topology: ProxyTopology | null;
	stats: ProxyStats | null;
};

const defaultFilters: ActivityFilters = {
	timeRange: "all",
	method: "ALL",
	host: "ALL",
	status: "ALL",
	duration: "ALL",
};

function timeRangeToMs(range: Exclude<TimeRange, "all">): number {
	const map: Record<Exclude<TimeRange, "all">, number> = {
		"5m": 5 * 60_000,
		"15m": 15 * 60_000,
		"30m": 30 * 60_000,
		"1h": 60 * 60_000,
		"6h": 6 * 60 * 60_000,
		"1d": 24 * 60 * 60_000,
		"1w": 7 * 24 * 60 * 60_000,
	};
	return map[range];
}

function matchesDuration(ms: number, filter: DurationFilter): boolean {
	switch (filter) {
		case "<100ms":
			return ms < 100;
		case "<500ms":
			return ms < 500;
		case "<1s":
			return ms < 1000;
		case ">1s":
			return ms >= 1000;
		default:
			return true;
	}
}

function VirtualRequestTable({ requests }: { requests: ProxyRequest[] }) {
	const parentRef = useRef<HTMLDivElement>(null);

	const rowVirtualizer = useVirtualizer({
		count: requests.length,
		getScrollElement: () => parentRef.current,
		estimateSize: () => 32,
		overscan: 20,
	});

	if (requests.length === 0) {
		return (
			<div className="flex-1 glass rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 flex items-center justify-center text-sm text-gray-400 dark:text-zinc-500">
				No requests match the current filters.
			</div>
		);
	}

	return (
		<div className="flex-1 min-h-0 glass rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 shadow-sm overflow-hidden flex flex-col">
			<div className="px-4 py-2 border-b border-gray-200/60 dark:border-zinc-800 flex items-center justify-between">
				<span className="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-zinc-400">
					Requests
				</span>
				<span className="text-[0.625rem] font-mono text-gray-400 dark:text-zinc-500">{requests.length} results</span>
			</div>

			<div className="grid grid-cols-[80px_60px_1fr_1fr_60px_70px] bg-gray-50 dark:bg-zinc-900 border-b border-gray-200/60 dark:border-zinc-800 text-xs font-mono">
				<div className="px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium">Time</div>
				<div className="px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium">Method</div>
				<div className="px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium">Host</div>
				<div className="px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium">Path</div>
				<div className="px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium text-right">Status</div>
				<div className="px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium text-right">Duration</div>
			</div>

			<div ref={parentRef} className="flex-1 overflow-y-auto">
				<div style={{ height: `${rowVirtualizer.getTotalSize()}px`, position: "relative" }}>
					{rowVirtualizer.getVirtualItems().map((virtualRow) => {
						const req = requests[virtualRow.index];
						return (
							<div
								key={virtualRow.key}
								data-index={virtualRow.index}
								className="grid grid-cols-[80px_60px_1fr_1fr_60px_70px] border-b border-gray-100/60 dark:border-zinc-800/60 hover:bg-gray-50 dark:hover:bg-zinc-800/50 text-xs font-mono"
								style={{
									position: "absolute",
									top: 0,
									left: 0,
									width: "100%",
									height: `${virtualRow.size}px`,
									transform: `translateY(${virtualRow.start}px)`,
								}}
							>
								<div className="px-3 py-1.5 text-gray-400 dark:text-zinc-500 truncate">{formatTime(req.timestamp)}</div>
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

export function ActivityPage({ topology, stats }: ActivityPageProps) {
	const { data: requests } = useActivityRequests();
	const [filters, setFilters] = useState<ActivityFilters>(defaultFilters);

	const uniqueHosts = useMemo(() => {
		if (!requests) return [];
		return [...new Set(requests.map((r) => r.hostname))].sort();
	}, [requests]);

	const filteredRequests = useMemo(() => {
		if (!requests) return [];
		return requests.filter((req) => {
			if (filters.timeRange !== "all") {
				const ms = timeRangeToMs(filters.timeRange);
				if (Date.now() - req.timestamp > ms) return false;
			}
			if (filters.method !== "ALL" && req.method !== filters.method) return false;
			if (filters.host !== "ALL" && req.hostname !== filters.host) return false;
			if (filters.status !== "ALL") {
				const century = Math.floor(req.status / 100);
				const expected = Number.parseInt(filters.status[0], 10);
				if (century !== expected) return false;
			}
			if (filters.duration !== "ALL") {
				if (!matchesDuration(req.durationMs, filters.duration)) return false;
			}
			return true;
		});
	}, [requests, filters]);

	return (
		<div className="flex flex-col gap-3 h-[calc(100vh-theme(spacing.14))]">
			<StatsBar topology={topology} stats={stats} />
			<FilterBar filters={filters} onFiltersChange={setFilters} hosts={uniqueHosts} />
			<VirtualRequestTable requests={filteredRequests} />
		</div>
	);
}
