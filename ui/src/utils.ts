export function statusColor(status: number): string {
	if (status < 300) return 'text-emerald-500';
	if (status < 400) return 'text-amber-500';
	if (status < 500) return 'text-orange-500';
	return 'text-red-500';
}

export function methodColor(method: string): string {
	switch (method) {
		case 'GET':
			return 'text-sky-500';
		case 'POST':
			return 'text-emerald-500';
		case 'PUT':
		case 'PATCH':
			return 'text-amber-500';
		case 'DELETE':
			return 'text-red-500';
		default:
			return 'text-gray-500';
	}
}

export function formatTime(ts: number): string {
	return new Date(ts).toLocaleTimeString('en-US', { hour12: false });
}
