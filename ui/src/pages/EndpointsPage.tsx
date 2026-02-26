import { type Endpoint, endpoints, type HttpMethod } from "../data/endpoints";

const methodColors: Record<HttpMethod, string> = {
	GET: "bg-sky-500/10 text-sky-500 border-sky-500/20",
	OPTIONS: "bg-amber-500/10 text-amber-500 border-amber-500/20",
};

function MethodBadge({ method }: { method: HttpMethod }) {
	return (
		<span className={`inline-flex px-2 py-0.5 rounded text-[0.625rem] font-bold border ${methodColors[method]}`}>
			{method}
		</span>
	);
}

function EndpointCard({ endpoint }: { endpoint: Endpoint }) {
	return (
		<div className="glass rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 shadow-sm overflow-hidden">
			<div className="px-4 py-3 flex items-center gap-3 border-b border-gray-200/60 dark:border-zinc-800">
				<MethodBadge method={endpoint.method} />
				<code className="text-xs font-semibold text-gray-900 dark:text-zinc-100">{endpoint.path}</code>
				<span className="ml-auto text-[0.625rem] font-mono text-gray-400 dark:text-zinc-500">
					→ {endpoint.responseType}
				</span>
			</div>
			<div className="px-4 py-3 space-y-3">
				<p className="text-xs leading-relaxed text-gray-600 dark:text-zinc-400">{endpoint.description}</p>
				{endpoint.params && endpoint.params.length > 0 && (
					<div>
						<div className="text-[0.625rem] font-semibold uppercase tracking-wider text-gray-500 dark:text-zinc-400 mb-2">
							Parameters
						</div>
						<div className="rounded-lg border border-gray-200/60 dark:border-zinc-800 overflow-hidden">
							<table className="w-full text-xs font-mono">
								<thead className="bg-gray-50 dark:bg-zinc-900">
									<tr>
										<th className="text-left px-3 py-1.5 text-gray-500 dark:text-zinc-400 font-medium">Name</th>
										<th className="text-left px-3 py-1.5 text-gray-500 dark:text-zinc-400 font-medium">Type</th>
										<th className="text-left px-3 py-1.5 text-gray-500 dark:text-zinc-400 font-medium">Default</th>
										<th className="text-left px-3 py-1.5 text-gray-500 dark:text-zinc-400 font-medium">Description</th>
									</tr>
								</thead>
								<tbody>
									{endpoint.params.map((param) => (
										<tr key={param.name} className="border-t border-gray-100 dark:border-zinc-800/60">
											<td className="px-3 py-1.5 text-indigo-500 font-medium">{param.name}</td>
											<td className="px-3 py-1.5 text-gray-500 dark:text-zinc-400">{param.type}</td>
											<td className="px-3 py-1.5 text-gray-400 dark:text-zinc-500">{param.default ?? "—"}</td>
											<td className="px-3 py-1.5 text-gray-600 dark:text-zinc-400">{param.description}</td>
										</tr>
									))}
								</tbody>
							</table>
						</div>
					</div>
				)}
			</div>
		</div>
	);
}

export function EndpointsPage() {
	return (
		<div className="max-w-3xl space-y-4">
			<div>
				<h1 className="text-sm font-semibold tracking-tight">API Endpoints</h1>
				<p className="text-xs text-gray-500 dark:text-zinc-400 mt-1">
					All endpoints are served at <code className="text-indigo-500">https://proxy.lvh.me</code>
				</p>
			</div>
			<div className="space-y-3">
				{endpoints.map((endpoint) => (
					<EndpointCard key={`${endpoint.method}-${endpoint.path}`} endpoint={endpoint} />
				))}
			</div>
		</div>
	);
}
