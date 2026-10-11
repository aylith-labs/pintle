import { afterEach, expect, test } from 'bun:test';
import { fetchRequests, fetchTopology } from '../src/api';

const originalFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = originalFetch;
});

test('topology and activity reject failed HTTP responses instead of treating them as data', async () => {
	globalThis.fetch = async (input) => {
		const path = String(input);
		return Response.json({ error: 'proxy_unavailable' }, { status: path.includes('topology') ? 503 : 500 });
	};
	await expect(fetchTopology()).rejects.toThrow('503');
	await expect(fetchRequests(1000)).rejects.toThrow('500');
});
