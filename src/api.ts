import { getDockerRoutes, getTraefikTarget } from "./docker-watcher";
import { getAllRoutes } from "./router";
import { getStaticRoutes } from "./static-routes";
import { getEdgeStats, getHostStats, getRecentRequests, getTotalRequests, getUptime } from "./stats";

const VITE_DEV_URL = "http://localhost:5175";

function json(data: unknown, status = 200): Response {
	return new Response(JSON.stringify(data), {
		status,
		headers: {
			"content-type": "application/json",
			"access-control-allow-origin": "*",
		},
	});
}

function handleTopology(): Response {
	const routes = getAllRoutes();
	const traefik = getTraefikTarget();
	const dockerRoutes = getDockerRoutes();
	const staticRoutes = getStaticRoutes();

	return json({
		sniRouter: { port: 9443, listenPort: 443 },
		httpsServer: { port: 9444 },
		httpRedirect: { port: 9080, redirectPort: 80 },
		traefik: {
			ip: traefik?.host ?? null,
			port: traefik?.port ?? 443,
			domains: ["*.internal.example"],
		},
		routes: routes.map((r) => ({
			hostname: r.hostname,
			path: r.path,
			target: r.target,
			stripPath: r.stripPath,
			source: r.source,
			containerName: r.containerName,
		})),
		containers: dockerRoutes.map((r) => ({
			name: r.containerName ?? "unknown",
			hostname: r.hostname,
			target: r.target,
			source: "docker" as const,
		})),
		staticRoutes: staticRoutes.map((r) => ({
			hostname: r.hostname,
			target: r.target,
			source: "static" as const,
		})),
	});
}

function handleStats(): Response {
	return json({
		uptime: getUptime(),
		totalRequests: getTotalRequests(),
		hostStats: getHostStats(),
		edgeStats: getEdgeStats(),
	});
}

function handleRequests(url: URL): Response {
	const limit = Number.parseInt(url.searchParams.get("limit") ?? "50", 10);
	return json(getRecentRequests(limit));
}

async function proxyToVite(req: Request): Promise<Response> {
	const url = new URL(req.url);
	const viteUrl = `${VITE_DEV_URL}${url.pathname}${url.search}`;
	try {
		return await fetch(viteUrl, {
			method: req.method,
			headers: req.headers,
			body: req.method !== "GET" && req.method !== "HEAD" ? req.body : undefined,
		});
	} catch {
		return new Response("Dashboard UI not running. Start with: cd ui && bun run dev", {
			status: 502,
			headers: { "content-type": "text/plain" },
		});
	}
}

export async function handleApiRequest(req: Request): Promise<Response> {
	const url = new URL(req.url);
	const { pathname } = url;

	// Handle CORS preflight
	if (req.method === "OPTIONS") {
		return new Response(null, {
			status: 204,
			headers: {
				"access-control-allow-origin": "*",
				"access-control-allow-methods": "GET, OPTIONS",
				"access-control-allow-headers": "content-type",
			},
		});
	}

	// API endpoints
	if (pathname === "/api/topology") return handleTopology();
	if (pathname === "/api/stats") return handleStats();
	if (pathname === "/api/requests") return handleRequests(url);
	if (pathname === "/api/health") return json({ status: "ok" });

	// Proxy to Vite dev server (dashboard UI)
	return proxyToVite(req);
}
