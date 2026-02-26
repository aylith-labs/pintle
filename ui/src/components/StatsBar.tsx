import { Clock, GitFork, Pulse, Warning } from "@phosphor-icons/react";
import type { ProxyStats, ProxyTopology } from "../types";

type StatsBarProps = {
	topology: ProxyTopology | null;
	stats: ProxyStats | null;
};

function formatUptime(seconds: number): string {
	if (seconds < 60) return `${seconds}s`;
	if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
	const h = Math.floor(seconds / 3600);
	const m = Math.floor((seconds % 3600) / 60);
	return `${h}h ${m}m`;
}

function StatCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
	return (
		<div className="glass flex items-center gap-3 rounded-lg border border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 shadow-sm overflow-hidden">
			<div className="flex items-center justify-center w-10 self-stretch rounded-l-lg bg-indigo-500/10 dark:bg-indigo-500/20 text-indigo-500 shrink-0">
				{icon}
			</div>
			<div className="px-2 py-2">
				<div className="text-[10px] uppercase tracking-wider text-gray-500 dark:text-zinc-400">{label}</div>
				<div className="text-sm font-semibold text-gray-900 dark:text-zinc-100">{value}</div>
			</div>
		</div>
	);
}

export function StatsBar({ topology, stats }: StatsBarProps) {
	const totalRoutes = topology?.routes.length ?? 0;
	const totalRequests = stats?.totalRequests ?? 0;
	const uptime = stats?.uptime ?? 0;

	const totalErrors = stats ? Object.values(stats.hostStats).reduce((sum, s) => sum + s.errorCount, 0) : 0;
	const errorRate = totalRequests > 0 ? ((totalErrors / totalRequests) * 100).toFixed(1) : "0.0";

	return (
		<div className="grid grid-cols-2 md:grid-cols-4 gap-2">
			<StatCard icon={<Pulse size={18} weight="bold" />} label="Total Requests" value={String(totalRequests)} />
			<StatCard icon={<Clock size={18} weight="bold" />} label="Uptime" value={formatUptime(uptime)} />
			<StatCard icon={<GitFork size={18} weight="bold" />} label="Active Routes" value={String(totalRoutes)} />
			<StatCard icon={<Warning size={18} weight="bold" />} label="Error Rate" value={`${errorRate}%`} />
		</div>
	);
}
