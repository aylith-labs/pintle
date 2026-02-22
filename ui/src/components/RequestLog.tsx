import type { ProxyRequest } from "../types";

type RequestLogProps = {
	requests: ProxyRequest[] | null;
};

function statusColor(status: number): string {
	if (status < 300) return "text-emerald-500";
	if (status < 400) return "text-amber-500";
	if (status < 500) return "text-orange-500";
	return "text-red-500";
}

function methodColor(method: string): string {
	switch (method) {
		case "GET":
			return "text-sky-500";
		case "POST":
			return "text-emerald-500";
		case "PUT":
		case "PATCH":
			return "text-amber-500";
		case "DELETE":
			return "text-red-500";
		default:
			return "text-gray-500";
	}
}

function formatTime(ts: number): string {
	return new Date(ts).toLocaleTimeString("en-US", { hour12: false });
}

export function RequestLog({ requests }: RequestLogProps) {
	if (!requests || requests.length === 0) {
		return (
			<div className="glass rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 p-6 text-center text-sm text-gray-400 dark:text-zinc-500">
				No requests yet. Make some requests to see them here.
			</div>
		);
	}

	return (
		<div className="glass rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 shadow-sm overflow-hidden">
			<div className="px-4 py-2.5 border-b border-gray-200/60 dark:border-zinc-800">
				<span className="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-zinc-400">
					Recent Requests
				</span>
			</div>
			<div className="max-h-[300px] overflow-y-auto">
				<table className="w-full text-xs font-mono">
					<thead className="sticky top-0 bg-gray-50 dark:bg-zinc-900 border-b border-gray-200/60 dark:border-zinc-800">
						<tr>
							<th className="text-left px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium">Time</th>
							<th className="text-left px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium">Method</th>
							<th className="text-left px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium">Host</th>
							<th className="text-left px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium">Path</th>
							<th className="text-right px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium">Status</th>
							<th className="text-right px-3 py-2 text-gray-500 dark:text-zinc-400 font-medium">Duration</th>
						</tr>
					</thead>
					<tbody>
						{requests.map((req, i) => (
							<tr
								key={`${req.timestamp}-${i}`}
								className="border-b border-gray-100/60 dark:border-zinc-800/60 hover:bg-gray-50 dark:hover:bg-zinc-800/50"
							>
								<td className="px-3 py-1.5 text-gray-400 dark:text-zinc-500">{formatTime(req.timestamp)}</td>
								<td className={`px-3 py-1.5 font-medium ${methodColor(req.method)}`}>{req.method}</td>
								<td className="px-3 py-1.5 text-gray-700 dark:text-zinc-300">{req.hostname}</td>
								<td className="px-3 py-1.5 text-gray-500 dark:text-zinc-400 max-w-[200px] truncate">{req.path}</td>
								<td className={`px-3 py-1.5 text-right font-medium ${statusColor(req.status)}`}>{req.status}</td>
								<td className="px-3 py-1.5 text-right text-gray-400 dark:text-zinc-500">
									{req.durationMs.toFixed(0)}ms
								</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>
		</div>
	);
}
