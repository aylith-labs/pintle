import { readFileSync, watchFile } from "node:fs";
import { parse } from "yaml";
import * as log from "./logger";
import type { Route, StaticRouteConfig } from "./types";

let currentRoutes: Route[] = [];
let onChange: (() => void) | null = null;

function loadFile(filePath: string): Route[] {
	try {
		const content = readFileSync(filePath, "utf-8");
		const parsed = parse(content) as { routes?: StaticRouteConfig[] } | null;

		if (!parsed?.routes) return [];

		return parsed.routes.map((r) => ({
			hostname: r.host,
			path: r.path ?? "/",
			target: r.target,
			stripPath: r.strip ?? false,
			source: "static" as const,
		}));
	} catch (err) {
		log.error(`Failed to load routes file: ${filePath}`, err);
		return [];
	}
}

export function getStaticRoutes(): Route[] {
	return currentRoutes;
}

export function initStaticRoutes(filePath: string, onUpdate: () => void): void {
	onChange = onUpdate;
	currentRoutes = loadFile(filePath);
	log.info(`Loaded ${currentRoutes.length} static route(s) from ${filePath}`);

	watchFile(filePath, { interval: 1000 }, () => {
		log.info("Routes file changed, reloading...");
		currentRoutes = loadFile(filePath);
		onChange?.();
	});
}
