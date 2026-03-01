import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchRequests, fetchStats, fetchTopology } from './api';
import type { ProxyRequest, ProxyStats, ProxyTopology } from './types';

function usePolling<T>(fetcher: () => Promise<T>, intervalMs: number): { data: T | null; isLoading: boolean } {
	const [data, setData] = useState<T | null>(null);
	const [isLoading, setIsLoading] = useState(true);
	const fetcherRef = useRef(fetcher);
	fetcherRef.current = fetcher;

	useEffect(() => {
		let active = true;

		async function poll() {
			try {
				const result = await fetcherRef.current();
				if (active) {
					setData(result);
					setIsLoading(false);
				}
			} catch {
				// Silently retry on next interval
			}
		}

		poll();
		const id = setInterval(poll, intervalMs);
		return () => {
			active = false;
			clearInterval(id);
		};
	}, [intervalMs]);

	return { data, isLoading };
}

export function useTopology() {
	return usePolling<ProxyTopology>(fetchTopology, 10_000);
}

export function useStats() {
	return usePolling<ProxyStats>(fetchStats, 3_000);
}

export function useRequests() {
	return usePolling<ProxyRequest[]>(() => fetchRequests(100), 2_000);
}

export function useActivityRequests() {
	return usePolling<ProxyRequest[]>(() => fetchRequests(1000), 2_000);
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

export function useFontSize() {
	const [size, setSize] = useState<number>(() => {
		const stored = localStorage.getItem('proxy-font-size');
		return stored ? Number(stored) : 100;
	});

	useEffect(() => {
		document.documentElement.style.fontSize = `${size}%`;
		localStorage.setItem('proxy-font-size', String(size));
	}, [size]);

	const increase = useCallback(() => setSize((s) => Math.min(s + 10, 150)), []);
	const decrease = useCallback(() => setSize((s) => Math.max(s - 10, 75)), []);
	const reset = useCallback(() => setSize(100), []);

	return { size, increase, decrease, reset };
}
