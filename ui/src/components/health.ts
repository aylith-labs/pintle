import type { RouteStats } from '../types';

export type HealthState = 'healthy' | 'warning' | 'error' | 'idle';

export function healthOf(stats: RouteStats | undefined): HealthState {
	if (!stats) return 'idle';
	const errorRate = stats.errorCount / Math.max(stats.totalRequests, 1);
	if (errorRate > 0.5) return 'error';
	if (errorRate > 0.1) return 'warning';
	return 'healthy';
}

export const HEALTH_STATES: HealthState[] = ['healthy', 'warning', 'error', 'idle'];

export const HEALTH_DOT: Record<HealthState, string> = {
	healthy: 'bg-emerald-500',
	warning: 'bg-amber-500',
	error: 'bg-red-500',
	idle: 'bg-gray-400 dark:bg-zinc-600',
};

export const HEALTH_LABEL: Record<HealthState, string> = {
	healthy: 'Healthy',
	warning: 'Warning',
	error: 'Error',
	idle: 'Idle',
};
