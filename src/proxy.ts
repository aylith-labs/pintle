import * as log from './logger';
import { resolve } from './router';
import { recordRequest } from './stats';

export async function handleRequest(req: Request): Promise<Response> {
	const url = new URL(req.url);
	const hostname = (req.headers.get('host') ?? '').split(':')[0];

	const match = resolve(hostname, url.pathname);
	if (!match) {
		log.route(req.method, hostname, url.pathname, 'no route', 404);
		recordRequest({
			timestamp: Date.now(),
			method: req.method,
			hostname,
			path: url.pathname,
			target: 'none',
			status: 404,
			durationMs: 0,
		});
		return new Response(`No route for ${hostname}`, { status: 404 });
	}

	const targetUrl = `${match.target}${match.rewrittenPath}${url.search}`;
	const startTime = performance.now();

	try {
		const headers = new Headers(req.headers);
		headers.set('x-forwarded-for', req.headers.get('x-real-ip') ?? '127.0.0.1');
		headers.set('x-forwarded-proto', 'https');
		headers.set('x-forwarded-host', hostname);

		const response = await fetch(targetUrl, {
			method: req.method,
			headers,
			body: req.method !== 'GET' && req.method !== 'HEAD' ? req.body : undefined,
			redirect: 'manual',
			decompress: false,
		});

		const durationMs = performance.now() - startTime;
		log.route(req.method, hostname, url.pathname, match.target, response.status);
		recordRequest({
			timestamp: Date.now(),
			method: req.method,
			hostname,
			path: url.pathname,
			target: match.target,
			status: response.status,
			durationMs,
		});

		// Copy response headers, remove hop-by-hop headers
		const respHeaders = new Headers(response.headers);
		respHeaders.delete('transfer-encoding');

		return new Response(response.body, {
			status: response.status,
			statusText: response.statusText,
			headers: respHeaders,
		});
	} catch (err) {
		const durationMs = performance.now() - startTime;
		log.error(`Proxy error: ${hostname}${url.pathname} -> ${match.target}`, err);
		recordRequest({
			timestamp: Date.now(),
			method: req.method,
			hostname,
			path: url.pathname,
			target: match.target,
			status: 502,
			durationMs,
		});
		return new Response(`Upstream unreachable: ${match.target}`, { status: 502 });
	}
}
