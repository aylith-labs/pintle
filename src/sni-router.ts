import { createServer, Socket } from "node:net";
import * as log from "./logger";

/**
 * Parse SNI (Server Name Indication) from a TLS ClientHello message.
 * Returns the hostname or null if not found.
 */
function parseSNI(buf: Buffer): string | null {
	// TLS record: type(1) + version(2) + length(2) + handshake
	if (buf.length < 5 || buf[0] !== 0x16) return null; // 0x16 = Handshake

	const recordLength = buf.readUInt16BE(3);
	if (buf.length < 5 + recordLength) return null;

	// Handshake: type(1) + length(3) + ...
	let pos = 5;
	if (buf[pos] !== 0x01) return null; // 0x01 = ClientHello

	pos += 4; // Skip handshake type + length

	// ClientHello: version(2) + random(32) + session_id_len(1) + session_id + ...
	pos += 2 + 32; // version + random

	if (pos >= buf.length) return null;
	const sessionIdLen = buf[pos];
	pos += 1 + sessionIdLen;

	// Cipher suites: length(2) + data
	if (pos + 2 > buf.length) return null;
	const cipherSuitesLen = buf.readUInt16BE(pos);
	pos += 2 + cipherSuitesLen;

	// Compression methods: length(1) + data
	if (pos >= buf.length) return null;
	const compressionLen = buf[pos];
	pos += 1 + compressionLen;

	// Extensions: total_length(2) + extension*
	if (pos + 2 > buf.length) return null;
	const extensionsLen = buf.readUInt16BE(pos);
	pos += 2;

	const extensionsEnd = pos + extensionsLen;

	while (pos + 4 <= extensionsEnd && pos + 4 <= buf.length) {
		const extType = buf.readUInt16BE(pos);
		const extLen = buf.readUInt16BE(pos + 2);
		pos += 4;

		if (extType === 0x0000) {
			// SNI extension
			// server_name_list_length(2) + server_name_type(1) + host_name_length(2) + hostname
			if (pos + 5 > buf.length) return null;
			const nameType = buf[pos + 2];
			if (nameType !== 0x00) return null; // 0x00 = host_name

			const nameLen = buf.readUInt16BE(pos + 3);
			if (pos + 5 + nameLen > buf.length) return null;

			return buf.subarray(pos + 5, pos + 5 + nameLen).toString("ascii");
		}

		pos += extLen;
	}

	return null;
}

type SniForwardTarget = {
	match: (hostname: string) => boolean;
	resolve: () => { host: string; port: number } | null;
	label: string;
};

type SniRouterConfig = {
	port: number;
	localTarget: { host: string; port: number };
	forwardTargets: SniForwardTarget[];
};

function pipeToTarget(clientSocket: Socket, data: Buffer, host: string, port: number): void {
	const upstream = new Socket();
	upstream.connect(port, host, () => {
		upstream.write(data); // Forward the initial TLS ClientHello
		clientSocket.pipe(upstream);
		upstream.pipe(clientSocket);
	});

	upstream.on("error", () => clientSocket.destroy());
	clientSocket.on("error", () => upstream.destroy());
}

export function startSniRouter(config: SniRouterConfig): void {
	const server = createServer((clientSocket: Socket) => {
		clientSocket.once("data", (data: Buffer) => {
			const sni = parseSNI(data);

			// Find a forward target for this SNI hostname
			const forwardRule = sni ? config.forwardTargets.find((t) => t.match(sni)) : null;

			if (forwardRule) {
				const target = forwardRule.resolve();
				if (target) {
					// TCP passthrough to external target (e.g., Traefik)
					pipeToTarget(clientSocket, data, target.host, target.port);
				} else {
					// Target not available (e.g., Traefik not running)
					clientSocket.destroy();
				}
			} else {
				// Forward to local Bun HTTPS server
				pipeToTarget(clientSocket, data, config.localTarget.host, config.localTarget.port);
			}
		});

		clientSocket.on("error", () => {});
	});

	server.listen(config.port, () => {
		log.info(`SNI router listening on :${config.port}`);
		for (const target of config.forwardTargets) {
			log.info(`  ${target.label} (passthrough, dynamic IP)`);
		}
		log.info(`  *.lvh.me -> localhost:${config.localTarget.port} (local TLS)`);
	});
}
