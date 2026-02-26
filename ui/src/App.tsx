import { BookOpen, ChartLine, FlowArrow, ListBullets, ListMagnifyingGlass, Plugs } from "@phosphor-icons/react";
import { useCallback, useEffect, useState } from "react";
import { SettingsMenu } from "./components/SettingsMenu";
import { useFontSize, useStats, useTheme, useTopology } from "./hooks";
import { ActivityPage } from "./pages/ActivityPage";
import { ArchitecturePage } from "./pages/ArchitecturePage";
import { DashboardPage } from "./pages/DashboardPage";
import { EndpointsPage } from "./pages/EndpointsPage";
import { GlossaryPage } from "./pages/GlossaryPage";

type Page = "dashboard" | "activity" | "architecture" | "glossary" | "endpoints";

const pages = ["dashboard", "activity", "architecture", "glossary", "endpoints"] as const satisfies readonly Page[];

function hashToPage(hash: string): Page {
	const path = hash.replace("#/", "") as Page;
	return pages.includes(path) ? path : "dashboard";
}

function useRoute(): { page: Page; navigate: (p: Page) => void } {
	const [page, setPage] = useState<Page>(() => hashToPage(window.location.hash));

	useEffect(() => {
		function onHashChange() {
			setPage(hashToPage(window.location.hash));
		}
		window.addEventListener("hashchange", onHashChange);
		return () => window.removeEventListener("hashchange", onHashChange);
	}, []);

	const navigate = useCallback((p: Page) => {
		window.location.hash = p === "dashboard" ? "#/" : `#/${p}`;
	}, []);

	return { page, navigate };
}

const navItems: { page: Page; label: string; icon: typeof ChartLine }[] = [
	{ page: "dashboard", label: "Dashboard", icon: ChartLine },
	{ page: "activity", label: "Activity", icon: ListMagnifyingGlass },
	{ page: "architecture", label: "Architecture", icon: BookOpen },
	{ page: "glossary", label: "Glossary", icon: ListBullets },
	{ page: "endpoints", label: "Endpoints", icon: Plugs },
];

export function App() {
	const { data: topology, isLoading: topoLoading } = useTopology();
	const { data: stats } = useStats();
	const { theme, setTheme } = useTheme();
	const { size: fontSize, increase, decrease, reset } = useFontSize();
	const { page, navigate } = useRoute();

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
				<div className="px-4 py-2 flex items-center justify-between">
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
										className={`flex items-center gap-1.5 px-4 py-2 rounded-md text-[0.6875rem] font-medium transition-colors ${
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

					<SettingsMenu
						theme={theme}
						setTheme={setTheme}
						fontSize={fontSize}
						onIncrease={increase}
						onDecrease={decrease}
						onReset={reset}
					/>
				</div>
			</header>

			{/* Content */}
			<main className="px-4 pt-4 pb-3 space-y-3">
				{page === "dashboard" && <DashboardPage topology={topology} stats={stats} />}
				{page === "activity" && <ActivityPage topology={topology} stats={stats} />}
				{page === "architecture" && <ArchitecturePage />}
				{page === "glossary" && <GlossaryPage />}
				{page === "endpoints" && <EndpointsPage />}
			</main>
		</div>
	);
}
