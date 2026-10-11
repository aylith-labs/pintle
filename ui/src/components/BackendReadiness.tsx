import { useEffect, useState } from 'react';
import type { BackendHealthSnapshot } from '../types';

const day = 86_400_000;
const labels = { ready: 'Ready', down: 'Down', unknown: 'Unknown' };

/** Probe observations are separate from the diagram's recorded request results. */
export function BackendReadiness({ snapshot }: { snapshot?: BackendHealthSnapshot }) {
	const [now, setNow] = useState(Date.now);
	useEffect(() => {
		const timer = setInterval(() => setNow(Date.now()), 1000);
		return () => clearInterval(timer);
	}, []);
	if (!snapshot) return null;
	const elapsed = snapshot.receivedAtMs === undefined ? 0 : Math.max(0, now - snapshot.receivedAtMs);
	const stale = snapshot.receivedAtMs === undefined || elapsed > 30_000;
	return (
		<section
			className="shrink-0 rounded-xl border border-gray-200 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 p-3"
			aria-label="Backend readiness"
		>
			<div className="flex flex-wrap items-center justify-between gap-2">
				<h2 className="text-sm font-semibold">Backend readiness</h2>
				<span className="text-xs text-gray-500 dark:text-zinc-400">Generation {snapshot.generation}</span>
			</div>
			<p className="mt-1 text-xs text-gray-500 dark:text-zinc-400">
				Latest configured GET probes. Request results in the diagram summarize past traffic.
			</p>
			{!snapshot.running && (
				<p className="mt-2 text-xs" role="status">
					Readiness checks are stopped. Backends are unavailable until checks resume.
				</p>
			)}
			{stale && (
				<p className="mt-2 text-xs" role="status">
					Waiting for current readiness data. Last-known observations are shown below.
				</p>
			)}
			{snapshot.backends.length === 0 ? (
				<p className="mt-2 text-xs">No active checks configured. Add health to an explicit static replica pool.</p>
			) : (
				<div className="mt-3 max-h-56 overflow-auto">
					<table className="w-full text-left text-xs">
						<thead>
							<tr>
								<th className="pr-4 pb-2">Route / pool</th>
								<th className="pr-4 pb-2">Backend</th>
								<th className="pr-4 pb-2">Last probe</th>
								<th className="pb-2">Checked</th>
							</tr>
						</thead>
						<tbody>
							{snapshot.backends.map((row) => {
								const age =
									row.lastCheckAgeMs === null ? null : Math.min(day, Math.max(0, row.lastCheckAgeMs) + elapsed);
								const capped = row.ageCapped || age === day;
								const state = snapshot.running ? row.state : 'unknown';
								return (
									<tr
										key={`${snapshot.generation}:${row.hostname}:${row.pool}:${row.path}:${row.targetOrigin}`}
										className="border-t border-gray-100 dark:border-zinc-800"
									>
										<td className="pr-4 py-2">
											<code>
												{row.hostname}
												{row.path}
											</code>
											<span className="block text-gray-500">{row.pool}</span>
										</td>
										<td className="pr-4 py-2">
											<code>{row.targetOrigin}</code>
										</td>
										<td className="pr-4 py-2">
											{stale && state !== 'unknown' ? 'Last known ' : ''}
											{labels[state]}
										</td>
										<td className="py-2">
											{age === null
												? 'Not checked yet'
												: capped
													? 'At least 24h ago'
													: `${Math.floor(age / 1000)}s ago`}
										</td>
									</tr>
								);
							})}
						</tbody>
					</table>
				</div>
			)}
		</section>
	);
}
