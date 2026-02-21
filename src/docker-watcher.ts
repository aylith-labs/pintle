import Docker from "dockerode";
import * as log from "./logger";
import type { Route } from "./types";

const docker = new Docker();

let currentRoutes: Route[] = [];
let onChange: (() => void) | null = null;
let rebuildTimer: ReturnType<typeof setTimeout> | null = null;

const NETWORK_NAME = process.env.DOCKER_NETWORK ?? "traefik";

function parseLabels(labels: Record<string, string>): {
	hosts: string[];
	port: number | null;
	path: string;
	strip: boolean;
} | null {
	const hostLabel = labels["proxy.host"];
	if (!hostLabel) return null;

	return {
		hosts: hostLabel.split(",").map((h) => h.trim()),
		port: labels["proxy.port"] ? Number.parseInt(labels["proxy.port"], 10) : null,
		path: labels["proxy.path"] ?? "/",
		strip: labels["proxy.strip"] === "true",
	};
}

async function discoverRoutes(): Promise<Route[]> {
	const containers = await docker.listContainers({
		filters: { label: ["proxy.host"] },
	});

	const routes: Route[] = [];

	for (const containerInfo of containers) {
		const parsed = parseLabels(containerInfo.Labels);
		if (!parsed) continue;

		// Get container IP on the shared network
		const networks = containerInfo.NetworkSettings?.Networks ?? {};
		const networkInfo = networks[NETWORK_NAME];

		if (!networkInfo?.IPAddress) {
			log.error(`Container ${containerInfo.Names[0]} has no IP on network '${NETWORK_NAME}'`);
			continue;
		}

		// Determine port: label > first exposed port
		let port = parsed.port;
		if (!port && containerInfo.Ports.length > 0) {
			port = containerInfo.Ports[0].PrivatePort;
		}
		if (!port) {
			log.error(`Container ${containerInfo.Names[0]} has no port configured`);
			continue;
		}

		const target = `http://${networkInfo.IPAddress}:${port}`;
		const name = containerInfo.Names[0]?.replace(/^\//, "") ?? "unknown";

		for (const hostname of parsed.hosts) {
			routes.push({
				hostname,
				path: parsed.path,
				target,
				stripPath: parsed.strip,
				source: "docker",
				containerName: name,
			});
		}
	}

	return routes;
}

function scheduleRebuild(): void {
	if (rebuildTimer) clearTimeout(rebuildTimer);
	rebuildTimer = setTimeout(async () => {
		try {
			currentRoutes = await discoverRoutes();
			onChange?.();
		} catch (err) {
			log.error("Failed to rebuild Docker routes", err);
		}
	}, 300);
}

export function getDockerRoutes(): Route[] {
	return currentRoutes;
}

export async function initDockerWatcher(onUpdate: () => void): Promise<void> {
	onChange = onUpdate;

	// Initial discovery
	try {
		currentRoutes = await discoverRoutes();
		log.info(`Discovered ${currentRoutes.length} Docker route(s) on network '${NETWORK_NAME}'`);
	} catch (err) {
		log.error("Failed initial Docker discovery", err);
	}

	// Watch for container events
	try {
		const stream = await docker.getEvents({
			filters: {
				type: ["container"],
				event: ["start", "stop", "die", "destroy"],
			},
		});

		stream.on("data", () => {
			scheduleRebuild();
		});

		stream.on("error", (err) => {
			log.error("Docker event stream error", err);
			// Reconnect after delay
			setTimeout(() => initDockerWatcher(onUpdate), 5000);
		});

		log.info("Watching Docker events for container changes");
	} catch (err) {
		log.error("Failed to watch Docker events", err);
	}
}
