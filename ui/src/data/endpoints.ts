export type HttpMethod = 'GET' | 'OPTIONS';

export type EndpointParam = {
	name: string;
	type: string;
	required: boolean;
	description: string;
	default?: string;
};

export type Endpoint = {
	method: HttpMethod;
	path: string;
	description: string;
	responseType: string;
	params?: EndpointParam[];
};

export const endpoints: Endpoint[] = [
	{
		method: 'GET',
		path: '/api/topology',
		description:
			'Returns the full proxy topology including SNI router config, HTTPS server, HTTP redirect, Traefik status, all active routes, Docker containers, and static routes.',
		responseType: 'ProxyTopology',
	},
	{
		method: 'GET',
		path: '/api/stats',
		description:
			'Returns proxy runtime statistics: uptime, total request count, per-host stats (request count, error count, avg duration), and edge stats.',
		responseType: 'ProxyStats',
	},
	{
		method: 'GET',
		path: '/api/requests',
		description:
			'Returns a list of recent proxied requests with timestamp, method, hostname, path, target, status code, and duration.',
		responseType: 'ProxyRequest[]',
		params: [
			{
				name: 'limit',
				type: 'number',
				required: false,
				description: 'Maximum number of requests to return',
				default: '50',
			},
		],
	},
	{
		method: 'GET',
		path: '/api/health',
		description: 'Simple health check endpoint. Returns OK status when the proxy is running.',
		responseType: '{ status: "ok" }',
	},
	{
		method: 'OPTIONS',
		path: '/api/*',
		description: 'CORS preflight handler. Returns allowed origins, methods (GET, OPTIONS), and headers (content-type).',
		responseType: '204 No Content',
	},
];
