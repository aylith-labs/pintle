import { afterEach, expect, test } from 'bun:test';
import { fetchRequests } from '../src/api';
import { createPollingResource } from '../src/polling';

const originalFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = originalFetch;
});

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason: Error) => void;
	const promise = new Promise<T>((ok, fail) => {
		resolve = ok;
		reject = fail;
	});
	return { promise, resolve, reject };
}

test('failed initial read is an error, while a successful empty activity list is empty data', async () => {
	const first = deferred<Response>();
	const second = deferred<Response>();
	let calls = 0;
	globalThis.fetch = async () => (++calls === 1 ? first.promise : second.promise);
	const resource = createPollingResource(
		(signal) => fetchRequests(1000, signal),
		() => {},
	);
	const initial = resource.poll();
	first.resolve(Response.json({ error: 'unavailable' }, { status: 503 }));
	await initial;
	expect(resource.state).toEqual({
		data: null,
		isLoading: false,
		isRefreshing: false,
		error: 'Proxy API request failed (503)',
	});
	const retry = resource.retry();
	second.resolve(Response.json([]));
	await retry;
	expect(resource.state).toEqual({ data: [], isLoading: false, isRefreshing: false, error: null });
	resource.dispose();
});

test('retry owns the latest response and leaves last-good data visible after a later failure', async () => {
	const old = deferred<string[]>();
	const current = deferred<string[]>();
	const failed = deferred<string[]>();
	const recovered = deferred<string[]>();
	const queue = [old, current, failed, recovered];
	const resource = createPollingResource(
		() => {
			const next = queue.shift();
			if (!next) throw new Error('unexpected extra read');
			return next.promise;
		},
		() => {},
	);
	const first = resource.poll();
	expect(resource.poll()).toBe(first); // interval must not overlap an in-flight read
	const retry = resource.retry();
	current.resolve(['new route']);
	await retry;
	old.resolve(['stale route']);
	await first;
	expect(resource.state.data).toEqual(['new route']);
	const refresh = resource.poll();
	failed.reject(new Error('network down'));
	await refresh;
	expect(resource.state).toEqual({ data: ['new route'], isLoading: false, isRefreshing: false, error: 'network down' });
	const afterFailure = resource.poll();
	expect(resource.state.error).toBe('network down');
	recovered.resolve(['recovered route']);
	await afterFailure;
	expect(resource.state.error).toBeNull();
	expect(resource.state.data).toEqual(['recovered route']);
	resource.dispose();
});

test('dispose ignores an outstanding response', async () => {
	const pending = deferred<string[]>();
	let notifications = 0;
	const resource = createPollingResource(
		() => pending.promise,
		() => {
			notifications++;
		},
	);
	const request = resource.poll();
	const beforeDispose = notifications;
	resource.dispose();
	pending.resolve(['late route']);
	await request;
	expect(resource.state.data).toBeNull();
	expect(notifications).toBe(beforeDispose);
});
