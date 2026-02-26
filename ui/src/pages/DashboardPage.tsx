import { FlowDiagram } from "../components/FlowDiagram";
import { StatsBar } from "../components/StatsBar";
import type { ProxyStats, ProxyTopology } from "../types";

type DashboardPageProps = {
	topology: ProxyTopology | null;
	stats: ProxyStats | null;
};

export function DashboardPage({ topology, stats }: DashboardPageProps) {
	return (
		<div className="flex flex-col gap-3 h-[calc(100vh-theme(spacing.14))]">
			<StatsBar topology={topology} stats={stats} />
			<FlowDiagram topology={topology} stats={stats} />
		</div>
	);
}
