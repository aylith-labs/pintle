import { BookOpen, ChartLine, FlowArrow, Monitor, Moon, Sun } from "@phosphor-icons/react";
import { useCallback, useEffect, useState } from "react";
import { useRequests, useStats, useTheme, useTopology } from "./hooks";
import { ArchitecturePage } from "./pages/ArchitecturePage";
import { DashboardPage } from "./pages/DashboardPage";

type Page = "dashboard" | "architecture";

function useRoute(): { page: Page; navigate: (p: Page) => void } {
	const [page, setPage] = useState<Page>(() => {
		const hash = window.location.hash;
		if (hash === "#/architecture") return "architecture";
		return "dashboard";
	});

	useEffect(() => {
		function onHashChange() {
			const hash = window.location.hash;
			setPage(hash === "#/architecture" ? "architecture" : "dashboard");
		}
		window.addEventListener("hashchange", onHashChange);
		return () => window.removeEventListener("hashchange", onHashChange);
	}, []);

	const navigate = useCallback((p: Page) => {
		window.location.hash = p === "dashboard" ? "#/" : `#/${p}`;
	}, []);

	return { page, navigate };
}

const themeIcons = {
	system: Monitor,
	light: Sun,
	dark: Moon,
};

const navItems: { page: Page; label: string; icon: typeof ChartLine }[] = [
	{ page: "dashboard", label: "Dashboard", icon: ChartLine },
	{ page: "architecture", label: "Architecture", icon: BookOpen },
];

export function App() {
	const { data: topology, isLoading: topoLoading } = useTopology();
	const { data: stats } = useStats();
	const { data: requests } = useRequests();
	const { theme, cycleTheme } = useTheme();
	const { page, navigate } = useRoute();

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
					<div className="flex items-center gap-4">
						<div className="flex items-center gap-2.5">
							<FlowArrow size={20} weight="bold" className="text-indigo-500" />
							<h1 className="text-sm font-semibold tracking-tight">local-proxy</h1>
						</div>

						{/* Navigation */}
						<nav className="flex items-center gap-1 ml-2">
							{navItems.map((item) => {
								const isActive = page === item.page;
								const Icon = item.icon;
								return (
									<button
										key={item.page}
										type="button"
										onClick={() => navigate(item.page)}
										className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-medium transition-colors ${
											isActive
												? "bg-indigo-500/10 text-indigo-600 dark:text-indigo-400"
												: "text-gray-500 dark:text-zinc-400 hover:bg-gray-100 dark:hover:bg-zinc-800"
										}`}
									>
										<Icon size={13} weight={isActive ? "bold" : "regular"} />
										{item.label}
									</button>
								);
							})}
						</nav>
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
				{page === "dashboard" && <DashboardPage topology={topology} stats={stats} requests={requests} />}
				{page === "architecture" && <ArchitecturePage />}
			</main>
		</div>
	);
}
