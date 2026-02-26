import { useCallback, useRef, useState } from "react";
import { glossary } from "../data/glossary";

type TooltipPos = {
	vertical: "above" | "below";
	left: number;
};

export function Abbr({ children, title }: { children: string; title?: string }) {
	const entry = glossary[children];
	const tooltipText = title ?? (entry ? `${entry.term} — ${entry.description}` : children);
	const ref = useRef<HTMLSpanElement>(null);
	const tooltipRef = useRef<HTMLSpanElement>(null);
	const [pos, setPos] = useState<TooltipPos>({ vertical: "above", left: 0 });

	const handleMouseEnter = useCallback(() => {
		if (!ref.current || !tooltipRef.current) return;
		const triggerRect = ref.current.getBoundingClientRect();
		const tooltipRect = tooltipRef.current.getBoundingClientRect();

		const vertical = triggerRect.top < 80 ? "below" : "above";

		// Center position
		const centerX = triggerRect.left + triggerRect.width / 2;
		const halfTooltip = tooltipRect.width / 2;
		const pad = 8;
		let left = 0;

		if (centerX - halfTooltip < pad) {
			// Overflows left — shift right
			left = pad - (centerX - halfTooltip);
		} else if (centerX + halfTooltip > window.innerWidth - pad) {
			// Overflows right — shift left
			left = window.innerWidth - pad - (centerX + halfTooltip);
		}

		setPos({ vertical, left });
	}, []);

	return (
		<span ref={ref} className="relative inline-block group/abbr" onMouseEnter={handleMouseEnter}>
			<span className="border-b border-dotted border-current cursor-help">{children}</span>
			<span
				ref={tooltipRef}
				style={{ transform: `translateX(calc(-50% + ${pos.left}px))` }}
				className={`absolute left-1/2 px-3 py-2 text-[11px] leading-relaxed bg-gray-900 dark:bg-zinc-100 text-white dark:text-zinc-900 rounded-lg shadow-lg opacity-0 group-hover/abbr:opacity-100 transition-opacity duration-150 pointer-events-none w-max max-w-[280px] z-50 text-center font-sans normal-case tracking-normal ${
					pos.vertical === "above" ? "bottom-full mb-1.5" : "top-full mt-1.5"
				}`}
			>
				{tooltipText}
			</span>
		</span>
	);
}
