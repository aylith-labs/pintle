import { ArrowCounterClockwise, GearSix, Monitor, Moon, Sun } from '@phosphor-icons/react';
import { useEffect, useRef, useState } from 'react';

type Theme = 'system' | 'light' | 'dark';

type SettingsMenuProps = {
	theme: Theme;
	setTheme: (t: Theme) => void;
	fontSize: number;
	onIncrease: () => void;
	onDecrease: () => void;
	onReset: () => void;
};

const themeOptions: { value: Theme; icon: typeof Sun; label: string }[] = [
	{ value: 'light', icon: Sun, label: 'Light' },
	{ value: 'dark', icon: Moon, label: 'Dark' },
	{ value: 'system', icon: Monitor, label: 'System' },
];

export function SettingsMenu({ theme, setTheme, fontSize, onIncrease, onDecrease, onReset }: SettingsMenuProps) {
	const [open, setOpen] = useState(false);
	const menuRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		if (!open) return;
		function handleClick(e: MouseEvent) {
			if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
				setOpen(false);
			}
		}
		document.addEventListener('mousedown', handleClick);
		return () => document.removeEventListener('mousedown', handleClick);
	}, [open]);

	return (
		<div ref={menuRef} className="relative">
			<button
				type="button"
				onClick={() => setOpen((o) => !o)}
				className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-gray-600 dark:text-zinc-400 hover:bg-gray-100 dark:hover:bg-zinc-800 transition-colors"
			>
				<GearSix size={14} weight="bold" />
			</button>

			{open && (
				<div className="absolute right-0 top-full mt-1 w-56 glass rounded-xl border border-gray-200/60 dark:border-zinc-800 bg-white/95 dark:bg-zinc-900/95 shadow-xl z-50 p-3 space-y-3">
					{/* Theme */}
					<div>
						<div className="text-[0.625rem] uppercase tracking-wider text-gray-500 dark:text-zinc-400 font-medium mb-1.5">
							Theme
						</div>
						<div className="flex gap-1">
							{themeOptions.map((opt) => {
								const Icon = opt.icon;
								const isActive = theme === opt.value;
								return (
									<button
										key={opt.value}
										type="button"
										onClick={() => setTheme(opt.value)}
										className={`flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-[0.625rem] font-medium transition-colors ${
											isActive
												? 'bg-indigo-500/10 text-indigo-600 dark:text-indigo-400'
												: 'text-gray-500 dark:text-zinc-400 hover:bg-gray-100 dark:hover:bg-zinc-800'
										}`}
									>
										<Icon size={13} weight={isActive ? 'bold' : 'regular'} />
										{opt.label}
									</button>
								);
							})}
						</div>
					</div>

					{/* Font Size */}
					<div>
						<div className="text-[0.625rem] uppercase tracking-wider text-gray-500 dark:text-zinc-400 font-medium mb-1.5">
							Font Size
						</div>
						<div className="flex items-center gap-1">
							<button
								type="button"
								onClick={onDecrease}
								disabled={fontSize <= 75}
								className="flex items-center justify-center w-8 h-8 rounded-lg text-gray-600 dark:text-zinc-400 hover:bg-gray-100 dark:hover:bg-zinc-800 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
								title="Decrease font size"
							>
								<span className="text-[0.625rem] font-bold leading-none">A</span>
							</button>
							<div className="flex-1 text-center text-xs font-mono text-gray-700 dark:text-zinc-300">{fontSize}%</div>
							<button
								type="button"
								onClick={onIncrease}
								disabled={fontSize >= 150}
								className="flex items-center justify-center w-8 h-8 rounded-lg text-gray-600 dark:text-zinc-400 hover:bg-gray-100 dark:hover:bg-zinc-800 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
								title="Increase font size"
							>
								<span className="text-sm font-bold leading-none">A</span>
							</button>
							<button
								type="button"
								onClick={onReset}
								disabled={fontSize === 100}
								className="flex items-center justify-center w-8 h-8 rounded-lg text-gray-500 dark:text-zinc-400 hover:bg-gray-100 dark:hover:bg-zinc-800 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
								title="Reset font size"
							>
								<ArrowCounterClockwise size={13} weight="bold" />
							</button>
						</div>
					</div>
				</div>
			)}
		</div>
	);
}
