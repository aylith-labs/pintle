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
	target: string;
	path?: string;
	strip?: boolean;
};

export type PassthroughConfig = {
	domain: string;
	target: 'traefik';
};

