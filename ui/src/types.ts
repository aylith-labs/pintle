export type ProxyRoute = {
	hostname: string;
	path: string;
	target: string;
	stripPath: boolean;
	source: "docker" | "static";
	containerName?: string;
};

export type ProxyContainer = {
	name: string;
	hostname: string;
	target: string;
	source: "docker";
};

export type ProxyStaticRoute = {
	hostname: string;
	target: string;
	source: "static";
};

export type ProxyTopology = {
	sniRouter: { port: number; listenPort: number };
	httpsServer: { port: number };
	httpRedirect: { port: number; redirectPort: number };
	traefik: {
		ip: string | null;
		port: number;
		domains: string[];
	};
	routes: ProxyRoute[];
	containers: ProxyContainer[];
	staticRoutes: ProxyStaticRoute[];
};

export type RouteStats = {
	totalRequests: number;
	errorCount: number;
	avgDurationMs: number;
	lastRequestAt: number;
};

export type ProxyStats = {
	uptime: number;
	totalRequests: number;
	hostStats: Record<string, RouteStats>;
	edgeStats: Record<string, RouteStats>;
};

export type ProxyRequest = {
	timestamp: number;
	method: string;
	hostname: string;
	path: string;
	target: string;
	status: number;
	durationMs: number;
};
