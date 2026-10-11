import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchRequests, fetchStats, fetchTopology } from './api';
import { createPollingResource, type PollingState } from './polling';
import type { ProxyRequest, ProxyStats, ProxyTopology } from './types';

function usePolling<T>(
	fetcher: (signal: AbortSignal) => Promise<T>,
	intervalMs: number,
): PollingState<T> & { retry: () => void } {
	const [state, setState] = useState<PollingState<T>>({
		data: null,
		isLoading: true,
		isRefreshing: false,
		error: null,
	});
	const fetcherRef = useRef(fetcher);
	const resourceRef = useRef<ReturnType<typeof createPollingResource<T>> | null>(null);
	fetcherRef.current = fetcher;

	useEffect(() => {
		const resource = createPollingResource((signal) => fetcherRef.current(signal), setState);
		resourceRef.current = resource;
		void resource.poll();
		const id = setInterval(() => {
			void resource.poll();
		}, intervalMs);
		return () => {
			clearInterval(id);
			resource.dispose();
			if (resourceRef.current === resource) resourceRef.current = null;
		};
	}, [intervalMs]);

	const retry = useCallback(() => {
		void resourceRef.current?.retry();
	}, []);
	return { ...state, retry };
}

export function useTopology() {
	return usePolling<ProxyTopology>(fetchTopology, 10_000);
}

export function useStats() {
	return usePolling<ProxyStats>(fetchStats, 3_000);
}

export function useRequests() {
	return usePolling<ProxyRequest[]>((signal) => fetchRequests(100, signal), 2_000);
}

export function useActivityRequests() {
	return usePolling<ProxyRequest[]>((signal) => fetchRequests(1000, signal), 2_000);
}

type Theme = 'system' | 'light' | 'dark';

export function useTheme() {
	const [theme, setTheme] = useState<Theme>(() => {
		if (typeof window === 'undefined') return 'system';
		return (localStorage.getItem('proxy-theme') as Theme) ?? 'system';
	});

	useEffect(() => {
		const root = document.documentElement;

		function apply(t: Theme) {
			if (t === 'dark' || (t === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
				root.classList.add('dark');
			} else {
				root.classList.remove('dark');
			}
		}

		apply(theme);
		localStorage.setItem('proxy-theme', theme);

		if (theme === 'system') {
			const mq = window.matchMedia('(prefers-color-scheme: dark)');
			const handler = () => apply('system');
			mq.addEventListener('change', handler);
			return () => mq.removeEventListener('change', handler);
		}
	}, [theme]);

	const cycleTheme = useCallback(() => {
		setTheme((prev) => {
			if (prev === 'system') return 'light';
			if (prev === 'light') return 'dark';
			return 'system';
		});
	}, []);

	return { theme, setTheme, cycleTheme };
}

export type FontSize = 100 | 120 | 150 | 200;

const FONT_SIZES: FontSize[] = [100, 120, 150, 200];

export function useFontSize() {
	const [size, setSize] = useState<FontSize>(() => {
		const stored = localStorage.getItem('proxy-font-size');
		const parsed = stored ? Number(stored) : 100;
		return FONT_SIZES.includes(parsed as FontSize) ? (parsed as FontSize) : 100;
	});

	useEffect(() => {
		const scale = size / 100;
		document.documentElement.style.fontSize = `${size}%`;
		document.documentElement.style.setProperty('--ui-scale', String(scale));
		localStorage.setItem('proxy-font-size', String(size));
	}, [size]);

	return { size, setSize, sizes: FONT_SIZES, scale: size / 100 };
}
