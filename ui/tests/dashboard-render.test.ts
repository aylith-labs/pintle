import { expect, mock, test } from 'bun:test';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import type { ProxyRequest } from '../src/types';

const topologyRead = { data: null, isLoading: false, error: 'Proxy API request failed (503)', retry: () => {} };
let activityRead: { data: ProxyRequest[] | null; isLoading: boolean; error: string | null; retry: () => void } = {
	data: null,
	isLoading: false,
	error: 'Proxy API request failed (500)',
	retry: () => {},
};

mock.module('../src/hooks', () => ({
	useTopology: () => topologyRead,
	useStats: () => ({ data: null }),
	useTheme: () => ({ theme: 'light', setTheme: () => {} }),
	useFontSize: () => ({ size: 100, setSize: () => {}, sizes: [100], scale: 1 }),
	useActivityRequests: () => activityRead,
}));

test('actual dashboard and activity components show failures rather than empty data', async () => {
	Object.assign(globalThis, { window: { location: { hash: '#/' } } });
	const { App } = await import('../src/App');
	const { ActivityPage } = await import('../src/pages/ActivityPage');
	const dashboard = renderToStaticMarkup(createElement(App));
	expect(dashboard).toContain('Proxy data is unavailable');
	expect(dashboard).toContain('Retry connection');
	expect(dashboard).not.toContain('Loading proxy data');
	const activity = renderToStaticMarkup(createElement(ActivityPage, { topology: null, stats: null }));
	expect(activity).toContain('Request activity could not load');
	expect(activity).toContain('Retry activity');
	expect(activity).not.toContain('No requests match the current filters');
	activityRead = { ...activityRead, data: [], error: null };
	const empty = renderToStaticMarkup(createElement(ActivityPage, { topology: null, stats: null }));
	expect(empty).toContain('No requests match the current filters');
	expect(empty).not.toContain('Request activity could not load');
	activityRead = { ...activityRead, error: 'Proxy API request failed (500)' };
	const stale = renderToStaticMarkup(createElement(ActivityPage, { topology: null, stats: null }));
	expect(stale).toContain('Showing the last known requests');
});
