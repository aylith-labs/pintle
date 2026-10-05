import { useEffect, useId, useRef, useState } from 'react';

export function FilterSelect({
	label,
	value,
	options,
	onChange,
}: {
	label: string;
	value: string;
	options: string[];
	onChange: (value: string) => void;
}) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState('');
	const root = useRef<HTMLDivElement>(null);
	const trigger = useRef<HTMLButtonElement>(null);
	const id = useId();
	const searchable = options.length > 10;
	const visible = options.filter((option) => option.toLowerCase().includes(query.toLowerCase()));
	function close(focus = false) {
		setOpen(false);
		setQuery('');
		if (focus) trigger.current?.focus();
	}
	useEffect(() => {
		if (!open) return;
		function outside(e: PointerEvent) {
			if (!root.current?.contains(e.target as Node)) {
				setOpen(false);
				setQuery('');
			}
		}
		document.addEventListener('pointerdown', outside);
		return () => document.removeEventListener('pointerdown', outside);
	}, [open]);
	useEffect(() => {
		if (open) root.current?.querySelector<HTMLElement>(searchable ? 'input' : '[aria-selected="true"]')?.focus();
	}, [open, searchable]);
	function display(option: string) {
		const index = option.toLowerCase().indexOf(query.toLowerCase());
		if (!query || index < 0) return option === 'ALL' || option === 'all' ? 'All' : option;
		return (
			<>
				{option.slice(0, index)}
				<mark>{option.slice(index, index + query.length)}</mark>
				{option.slice(index + query.length)}
			</>
		);
	}
	return (
		<div ref={root} className="relative flex items-center gap-1.5 text-xs">
			<span id={`${id}-label`} className="text-gray-500 dark:text-zinc-400">
				{label}
			</span>
			<button
				ref={trigger}
				type="button"
				aria-labelledby={`${id}-label ${id}-value`}
				aria-haspopup="listbox"
				aria-expanded={open}
				aria-controls={id}
				className="bg-gray-50 dark:bg-zinc-800 border border-gray-200/60 dark:border-zinc-700 rounded px-2 py-1 text-xs text-gray-900 dark:text-zinc-100 outline-none focus:border-indigo-500"
				onClick={() => {
					setQuery('');
					setOpen(!open);
				}}
				onKeyDown={(e) => {
					if (e.key === 'ArrowDown') {
						e.preventDefault();
						setOpen(true);
					}
					if (e.key === 'Escape') close(true);
				}}
			>
				<span id={`${id}-value`}>{value === 'ALL' || value === 'all' ? 'All' : value}</span>{' '}
				<span aria-hidden="true">⌄</span>
			</button>
			{open && (
				<div className="absolute left-0 top-full mt-1 z-50 min-w-48 max-w-[min(22rem,85vw)] rounded-lg border border-gray-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 p-1 shadow-lg">
					{searchable && (
						<input
							aria-label={`Search ${label}`}
							type="search"
							value={query}
							onChange={(e) => setQuery(e.target.value)}
							className="mb-1 w-full rounded border border-gray-200 dark:border-zinc-700 p-2 bg-transparent outline-none focus:border-indigo-500"
							onKeyDown={(e) => {
								if (e.key === 'Escape') {
									e.preventDefault();
									close(true);
								}
								if (e.key === 'ArrowDown') {
									e.preventDefault();
									root.current?.querySelector<HTMLElement>('[role="option"]')?.focus();
								}
							}}
						/>
					)}
					<div id={id} role="listbox" aria-labelledby={`${id}-label`} className="max-h-64 overflow-auto">
						{visible.map((option, index) => (
							<button
								key={option}
								type="button"
								role="option"
								aria-selected={value === option}
								className="block w-full rounded border border-transparent px-2 py-2 text-left outline-none focus:border-indigo-500 hover:bg-gray-100 dark:hover:bg-zinc-800"
								onClick={() => {
									onChange(option);
									close(true);
								}}
								onKeyDown={(e) => {
									if (e.key === 'Escape') {
										e.preventDefault();
										close(true);
									}
									if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(e.key)) {
										e.preventDefault();
										const buttons = root.current?.querySelectorAll<HTMLElement>('[role="option"]');
										buttons?.[
											e.key === 'Home'
												? 0
												: e.key === 'End'
													? visible.length - 1
													: (index + (e.key === 'ArrowDown' ? 1 : visible.length - 1)) % visible.length
										]?.focus();
									}
								}}
							>
								<span className="flex justify-between gap-3">
									<span>{display(option)}</span>
									<span aria-hidden="true">{value === option ? '✓' : ''}</span>
								</span>
								<span className="block text-[0.625rem] text-gray-500 dark:text-zinc-400">
									{option === 'ALL' || option === 'all'
										? `Every ${label.toLowerCase()} value`
										: `Filter by ${label.toLowerCase()}`}
								</span>
							</button>
						))}
						{visible.length === 0 && <p className="p-2 text-gray-500">No matching hosts</p>}
					</div>
				</div>
			)}
		</div>
	);
}
