import { FlowDiagram } from "../components/FlowDiagram";
import { RequestLog } from "../components/RequestLog";
import { StatsBar } from "../components/StatsBar";
import type { ProxyRequest, ProxyStats, ProxyTopology } from "../types";

type DashboardPageProps = {
	topology: ProxyTopology | null;
	stats: ProxyStats | null;
	requests: ProxyRequest[] | null;
};

export function DashboardPage({ topology, stats, requests }: DashboardPageProps) {
	return (
		<div className="flex flex-col gap-3 h-[calc(100vh-theme(spacing.14))]">
			<StatsBar topology={topology} stats={stats} />
			<FlowDiagram topology={topology} stats={stats} />
			<RequestLog requests={requests} />
		</div>
	);
}
