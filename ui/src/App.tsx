import { FlowArrow, Monitor, Moon, Sun } from "@phosphor-icons/react";
import { FlowDiagram } from "./components/FlowDiagram";
import { RequestLog } from "./components/RequestLog";
import { StatsBar } from "./components/StatsBar";
import { useRequests, useStats, useTheme, useTopology } from "./hooks";

const themeIcons = {
	system: Monitor,
	light: Sun,
	dark: Moon,
};

export function App() {
	const { data: topology, isLoading: topoLoading } = useTopology();
	const { data: stats } = useStats();
	const { data: requests } = useRequests();
	const { theme, cycleTheme } = useTheme();

	const ThemeIcon = themeIcons[theme];

	if (topoLoading) {
		return (
			<div className="min-h-screen bg-gray-50 dark:bg-zinc-950 flex items-center justify-center">
				<div className="text-sm text-gray-400 dark:text-zinc-500 font-mono animate-pulse">Loading proxy data...</div>
			</div>
		);
	}

	return (
		<div className="min-h-screen bg-gray-50 dark:bg-zinc-950 text-gray-900 dark:text-zinc-100">
			{/* Header */}
			<header className="border-b border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 glass sticky top-0 z-50">
				<div className="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between">
					<div className="flex items-center gap-2.5">
						<FlowArrow size={20} weight="bold" className="text-indigo-500" />
						<h1 className="text-sm font-semibold tracking-tight">local-proxy</h1>
					</div>
					<button
						type="button"
						onClick={cycleTheme}
						className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-gray-600 dark:text-zinc-400 hover:bg-gray-100 dark:hover:bg-zinc-800 transition-colors"
					>
						<ThemeIcon size={14} weight="bold" />
						<span className="uppercase tracking-wider">{theme}</span>
					</button>
				</div>
			</header>

			{/* Content */}
			<main className="max-w-7xl mx-auto px-4 py-6 space-y-6">
				<StatsBar topology={topology} stats={stats} />
				<FlowDiagram topology={topology} stats={stats} />
				<RequestLog requests={requests} />
			</main>
		</div>
	);
}
