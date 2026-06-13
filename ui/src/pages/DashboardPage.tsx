import { FlowDiagram } from '../components/FlowDiagram';
import { StatsBar } from '../components/StatsBar';
import type { ProxyStats, ProxyTopology } from '../types';

type DashboardPageProps = {
	topology: ProxyTopology | null;
	stats: ProxyStats | null;
	scale: number;
};

export function DashboardPage({ topology, stats, scale }: DashboardPageProps) {
	return (
		<div className="flex flex-col gap-3 h-full">
			<StatsBar topology={topology} stats={stats} />
			<FlowDiagram topology={topology} stats={stats} scale={scale} />
		</div>
	);
}
