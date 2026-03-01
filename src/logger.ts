const colors = {
	reset: '\x1b[0m',
	dim: '\x1b[2m',
	green: '\x1b[32m',
	yellow: '\x1b[33m',
	blue: '\x1b[34m',
	magenta: '\x1b[35m',
	cyan: '\x1b[36m',
	red: '\x1b[31m',
};

function timestamp(): string {
	return new Date().toLocaleTimeString('en-US', { hour12: false });
}

export function info(msg: string): void {
	console.log(`${colors.dim}${timestamp()}${colors.reset} ${colors.cyan}INFO${colors.reset}  ${msg}`);
}

export function route(method: string, host: string, path: string, target: string, status: number): void {
	const color = status < 400 ? colors.green : status < 500 ? colors.yellow : colors.red;
	console.log(
		`${colors.dim}${timestamp()}${colors.reset} ${color}${status}${colors.reset}   ${colors.dim}${method}${colors.reset} ${host}${path} ${colors.dim}->${colors.reset} ${target}`,
	);
}

export function routeChange(action: 'add' | 'remove', hostname: string, path: string, target: string): void {
	const symbol = action === 'add' ? `${colors.green}+` : `${colors.red}-`;
	console.log(
		`${colors.dim}${timestamp()}${colors.reset} ${symbol}${colors.reset}     ${hostname}${path} ${colors.dim}->${colors.reset} ${target}`,
	);
}

export function warn(msg: string): void {
	console.log(`${colors.dim}${timestamp()}${colors.reset} ${colors.yellow}WARN${colors.reset}  ${msg}`);
}

export function error(msg: string, err?: unknown): void {
	console.error(`${colors.dim}${timestamp()}${colors.reset} ${colors.red}ERROR${colors.reset} ${msg}`, err ?? '');
}
