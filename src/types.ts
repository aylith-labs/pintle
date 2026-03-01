export type Route = {
	hostname: string;
	path: string;
	target: string;
	stripPath: boolean;
	source: 'docker' | 'static' | 'traefik' | 'caddy';
	containerName?: string;
};

export type StaticRouteConfig = {
	host: string;
	target: string | number;
	path?: string;
	strip?: boolean;
};

export type PassthroughConfig = {
	domain: string;
	target: 'traefik';
};

export type TcpRoute = {
	hostname: string;
	targetHost: string;
	targetPort: number;
	listenPort: number;
	source: 'docker' | 'static' | 'traefik';
	containerName?: string;
};

export type StaticTcpRouteConfig = {
	host: string;
	target: string | number;
	listen: number;
};
