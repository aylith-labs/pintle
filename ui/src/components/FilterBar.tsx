import type { ActivityFilters, DurationFilter, MethodFilter, StatusFilter, TimeRange } from "../types";

type FilterBarProps = {
	filters: ActivityFilters;
	onFiltersChange: (filters: ActivityFilters) => void;
	hosts: string[];
};

function FilterSelect({
	label,
	value,
	options,
	onChange,
}: {
	label: string;
	value: string;
	options: string[];
	onChange: (value: string) => void;
}) {
	return (
		<label className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider text-gray-500 dark:text-zinc-400">
			{label}
			<select
				value={value}
				onChange={(e) => onChange(e.target.value)}
				className="bg-gray-50 dark:bg-zinc-800 border border-gray-200/60 dark:border-zinc-700 rounded px-2 py-1 text-xs font-mono text-gray-900 dark:text-zinc-100 outline-none focus:ring-1 focus:ring-indigo-500/50"
			>
				{options.map((opt) => (
					<option key={opt} value={opt}>
						{opt === "ALL" ? "All" : opt}
					</option>
				))}
			</select>
		</label>
	);
}

export function FilterBar({ filters, onFiltersChange, hosts }: FilterBarProps) {
	function update<K extends keyof ActivityFilters>(key: K, value: ActivityFilters[K]) {
		onFiltersChange({ ...filters, [key]: value });
	}

	return (
		<div className="glass rounded-lg border border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 px-3 py-2 flex flex-wrap items-center gap-3">
			<FilterSelect
				label="Time"
				value={filters.timeRange}
				options={["all", "5m", "15m", "30m", "1h", "6h", "1d", "1w"]}
				onChange={(v) => update("timeRange", v as TimeRange)}
			/>
			<FilterSelect
				label="Method"
				value={filters.method}
				options={["ALL", "GET", "POST", "PUT", "PATCH", "DELETE"]}
				onChange={(v) => update("method", v as MethodFilter)}
			/>
			<FilterSelect label="Host" value={filters.host} options={["ALL", ...hosts]} onChange={(v) => update("host", v)} />
			<FilterSelect
				label="Status"
				value={filters.status}
				options={["ALL", "2xx", "3xx", "4xx", "5xx"]}
				onChange={(v) => update("status", v as StatusFilter)}
			/>
			<FilterSelect
				label="Duration"
				value={filters.duration}
				options={["ALL", "<100ms", "<500ms", "<1s", ">1s"]}
				onChange={(v) => update("duration", v as DurationFilter)}
			/>
		</div>
	);
}
