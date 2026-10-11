export type PollingState<T> = {
	data: T | null;
	isLoading: boolean;
	isRefreshing: boolean;
	error: string | null;
};

// The same resource drives scheduled reads and the user's retry action. Only
// the newest request may publish data, errors, or completion state.
export function createPollingResource<T>(
	fetcher: (signal: AbortSignal) => Promise<T>,
	onChange: (state: PollingState<T>) => void,
) {
	let state: PollingState<T> = { data: null, isLoading: true, isRefreshing: false, error: null };
	let controller: AbortController | null = null;
	let inFlight: Promise<void> | null = null;
	let generation = 0;
	let disposed = false;

	function publish(next: PollingState<T>) {
		state = next;
		onChange(next);
	}

	function read(force: boolean): Promise<void> {
		if (disposed) return Promise.resolve();
		if (inFlight && !force) return inFlight;
		controller?.abort();
		const request = new AbortController();
		controller = request;
		const current = ++generation;
		publish({
			data: state.data,
			isLoading: state.data === null,
			isRefreshing: state.data !== null,
			error: state.data === null ? null : state.error,
		});
		const work = Promise.resolve()
			.then(() => fetcher(request.signal))
			.then((data) => {
				if (disposed || current !== generation) return;
				publish({ data, isLoading: false, isRefreshing: false, error: null });
			})
			.catch((error: unknown) => {
				if (disposed || current !== generation) return;
				publish({
					data: state.data,
					isLoading: false,
					isRefreshing: false,
					error: error instanceof Error ? error.message : 'Request failed',
				});
			})
			.finally(() => {
				if (current !== generation) return;
				controller = null;
				inFlight = null;
			});
		inFlight = work;
		return work;
	}

	return {
		get state() {
			return state;
		},
		poll: () => read(false),
		retry: () => read(true),
		dispose: () => {
			disposed = true;
			generation++;
			controller?.abort();
			controller = null;
			inFlight = null;
		},
	};
}
