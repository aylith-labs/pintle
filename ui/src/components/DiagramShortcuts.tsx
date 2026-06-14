import { KeyboardIcon } from '@phosphor-icons/react';
import { Panel, useReactFlow, useStore } from '@xyflow/react';
import { useEffect, useRef } from 'react';
import { isEditableTarget } from '../utils';

const FIT_OPTIONS = { padding: 0.3 };

type Shortcut = { keys: string[]; label: string };

const KEYBOARD_SHORTCUTS: Shortcut[] = [
	{ keys: ['0'], label: 'Fit view' },
	{ keys: ['1'], label: 'Reset zoom (100%)' },
	{ keys: ['+'], label: 'Zoom in' },
	{ keys: ['−'], label: 'Zoom out' },
	{ keys: ['f'], label: 'Focus search' },
	{ keys: ['Space', 'drag'], label: 'Pan' },
];

const MOUSE_SHORTCUTS: Shortcut[] = [
	{ keys: ['Scroll'], label: 'Zoom' },
	{ keys: ['Drag'], label: 'Pan canvas' },
	{ keys: ['Double-click'], label: 'Zoom in' },
	{ keys: ['Drag node'], label: 'Move node' },
	{ keys: ['Click group'], label: 'Expand / collapse' },
];

function Kbd({ children }: { children: string }) {
	return (
		<kbd className="rounded border border-gray-200 dark:border-zinc-600 bg-gray-100 dark:bg-zinc-700/70 px-1.5 py-px text-[0.625rem] font-medium leading-none text-gray-500 dark:text-zinc-300">
			{children}
		</kbd>
	);
}

function ShortcutRow({ shortcut }: { shortcut: Shortcut }) {
	return (
		<div className="flex items-center justify-between gap-6">
			<span className="text-gray-600 dark:text-zinc-300">{shortcut.label}</span>
			<span className="flex items-center gap-1">
				{shortcut.keys.map((key, index) => (
					<span key={key} className="flex items-center gap-1">
						{index > 0 && <span className="text-gray-400 dark:text-zinc-500 text-[0.625rem]">+</span>}
						<Kbd>{key}</Kbd>
					</span>
				))}
			</span>
		</div>
	);
}

export function DiagramShortcuts() {
	const { zoomIn, zoomOut, zoomTo, fitView } = useReactFlow();
	const width = useStore((state) => state.width);
	const height = useStore((state) => state.height);
	const prevSize = useRef({ width: 0, height: 0 });

	// Re-fit when the pane regains size after collapsing to zero (HMR/layout
	// reflows leave the viewport fitted to nothing -> blank grid otherwise).
	useEffect(() => {
		const wasEmpty = prevSize.current.width === 0 || prevSize.current.height === 0;
		if (wasEmpty && width > 0 && height > 0) {
			fitView(FIT_OPTIONS);
		}
		prevSize.current = { width, height };
	}, [width, height, fitView]);

	useEffect(() => {
		function onKeyDown(event: KeyboardEvent) {
			if (event.ctrlKey || event.metaKey || event.altKey) return;
			if (isEditableTarget(document.activeElement)) return;
			switch (event.key) {
				case '0':
					event.preventDefault();
					fitView(FIT_OPTIONS);
					break;
				case '1':
					event.preventDefault();
					zoomTo(1, { duration: 200 });
					break;
				case '+':
				case '=':
					event.preventDefault();
					zoomIn();
					break;
				case '-':
				case '_':
					event.preventDefault();
					zoomOut();
					break;
			}
		}
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	}, [zoomIn, zoomOut, zoomTo, fitView]);

	return (
		<Panel position="bottom-right" className="!m-2">
			<div className="group relative">
				<button
					type="button"
					aria-label="Keyboard shortcuts"
					className="flex items-center justify-center rounded-lg border border-gray-200 dark:border-zinc-700 bg-white/80 dark:bg-zinc-900/80 p-1.5 text-gray-600 dark:text-zinc-400 shadow-lg hover:text-indigo-500 dark:hover:text-indigo-400 transition-colors"
				>
					<KeyboardIcon size={16} weight="bold" />
				</button>
				<div className="invisible absolute bottom-full right-0 mb-2 w-64 origin-bottom-right translate-y-1 scale-95 opacity-0 transition-all duration-150 group-hover:visible group-hover:translate-y-0 group-hover:scale-100 group-hover:opacity-100 group-focus-within:visible group-focus-within:translate-y-0 group-focus-within:scale-100 group-focus-within:opacity-100">
					<div className="glass rounded-xl border border-gray-200/60 dark:border-zinc-700/60 bg-white/95 dark:bg-zinc-900/95 p-3 shadow-xl">
						<div className="text-[0.625rem] uppercase tracking-wider text-gray-400 dark:text-zinc-500 mb-1.5">
							Keyboard
						</div>
						<div className="flex flex-col gap-1.5 text-xs">
							{KEYBOARD_SHORTCUTS.map((shortcut) => (
								<ShortcutRow key={shortcut.label} shortcut={shortcut} />
							))}
						</div>
						<div className="text-[0.625rem] uppercase tracking-wider text-gray-400 dark:text-zinc-500 mt-3 mb-1.5">
							Mouse
						</div>
						<div className="flex flex-col gap-1.5 text-xs">
							{MOUSE_SHORTCUTS.map((shortcut) => (
								<ShortcutRow key={shortcut.label} shortcut={shortcut} />
							))}
						</div>
					</div>
				</div>
			</div>
		</Panel>
	);
}
