import * as log from "./logger";
import { resolve } from "./router";

export async function handleRequest(req: Request): Promise<Response> {
	const url = new URL(req.url);
	const hostname = (req.headers.get("host") ?? "").split(":")[0];

	const match = resolve(hostname, url.pathname);
	if (!match) {
		log.route(req.method, hostname, url.pathname, "no route", 404);
		return new Response(`No route for ${hostname}`, { status: 404 });
	}

	const targetUrl = `${match.target}${match.rewrittenPath}${url.search}`;

	try {
		const headers = new Headers(req.headers);
		headers.set("x-forwarded-for", req.headers.get("x-real-ip") ?? "127.0.0.1");
		headers.set("x-forwarded-proto", "https");
		headers.set("x-forwarded-host", hostname);

		const response = await fetch(targetUrl, {
			method: req.method,
			headers,
			body: req.method !== "GET" && req.method !== "HEAD" ? req.body : undefined,
			redirect: "manual",
		});

		log.route(req.method, hostname, url.pathname, match.target, response.status);

		// Copy response headers
		const respHeaders = new Headers(response.headers);
		// Remove hop-by-hop headers
		respHeaders.delete("transfer-encoding");

		return new Response(response.body, {
			status: response.status,
			statusText: response.statusText,
			headers: respHeaders,
		});
	} catch (err) {
		log.error(`Proxy error: ${hostname}${url.pathname} -> ${match.target}`, err);
		return new Response(`Upstream unreachable: ${match.target}`, { status: 502 });
	}
}
