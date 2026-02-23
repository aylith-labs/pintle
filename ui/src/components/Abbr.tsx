import { glossary } from "../data/glossary";

export function Abbr({ children, title }: { children: string; title?: string }) {
	const entry = glossary[children];
	const tooltipText = title ?? (entry ? `${entry.term} — ${entry.description}` : children);

	return (
		<span className="abbr-tooltip relative inline-block group/abbr">
			<span className="border-b border-dotted border-current cursor-help">{children}</span>
			<span className="absolute bottom-full left-1/2 -translate-x-1/2 mb-1.5 px-3 py-2 text-[11px] leading-relaxed bg-gray-900 dark:bg-zinc-100 text-white dark:text-zinc-900 rounded-lg shadow-lg opacity-0 group-hover/abbr:opacity-100 transition-opacity duration-150 pointer-events-none w-max max-w-[280px] z-50 text-center font-sans">
				{tooltipText}
			</span>
		</span>
	);
}
