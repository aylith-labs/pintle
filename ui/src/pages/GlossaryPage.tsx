import { glossary } from '../data/glossary';

export function GlossaryPage() {
	return (
		<div className="max-w-3xl space-y-4">
			<h1 className="text-sm font-semibold tracking-tight">Glossary</h1>
			<div className="glass rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 shadow-sm overflow-hidden">
				<div className="divide-y divide-gray-100 dark:divide-zinc-800/60">
					{Object.entries(glossary).map(([abbr, entry]) => (
						<div key={abbr} className="flex gap-4 px-4 py-3">
							<span className="font-mono text-xs font-semibold text-indigo-500 min-w-[70px] shrink-0">{abbr}</span>
							<div className="text-xs leading-relaxed">
								<span className="font-medium text-gray-900 dark:text-zinc-200">{entry.term}</span>
								<span className="text-gray-400 dark:text-zinc-500"> — </span>
								<span className="text-gray-600 dark:text-zinc-400">{entry.description}</span>
							</div>
						</div>
					))}
				</div>
			</div>
		</div>
	);
}
