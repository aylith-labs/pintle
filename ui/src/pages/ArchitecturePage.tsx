import { ArrowRight, FlowArrow } from "@phosphor-icons/react";
import { Abbr } from "../components/Abbr";

function Section({ title, children }: { title: string; children: React.ReactNode }) {
	return (
		<section className="space-y-3">
			<h2 className="text-sm font-semibold text-gray-900 dark:text-zinc-100 tracking-tight">{title}</h2>
			<div className="text-xs leading-relaxed text-gray-600 dark:text-zinc-400 space-y-2">{children}</div>
		</section>
	);
}

function DiagramRow({ left, arrow, right, note }: { left: string; arrow?: string; right: string; note?: string }) {
	return (
		<div className="flex items-center gap-2 font-mono text-[11px]">
			<span className="text-gray-900 dark:text-zinc-100 min-w-[120px]">{left}</span>
			<ArrowRight size={12} className="text-indigo-400 shrink-0" />
			{arrow && <span className="text-indigo-400 text-[10px]">{arrow}</span>}
			{arrow && <ArrowRight size={12} className="text-indigo-400 shrink-0" />}
			<span className="text-gray-900 dark:text-zinc-100">{right}</span>
			{note && <span className="text-gray-400 dark:text-zinc-500 ml-2">({note})</span>}
		</div>
	);
}

export function ArchitecturePage() {
	return (
		<div className="space-y-8 max-w-3xl">
			{/* Header with link back */}
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-2">
					<FlowArrow size={18} weight="bold" className="text-indigo-500" />
					<h1 className="text-sm font-semibold tracking-tight">Architecture</h1>
				</div>
				<a
					href="#/"
					className="text-[11px] font-medium text-indigo-500 hover:text-indigo-400 transition-colors flex items-center gap-1"
				>
					View Live Dashboard
					<ArrowRight size={12} />
				</a>
			</div>

			{/* Overview */}
			<Section title="Overview">
				<p>
					local-proxy is a <Abbr>HTTPS</Abbr> reverse proxy for local development. It routes <code>*.lvh.me</code>{" "}
					domains through a Bun-powered server and passes <code>*.internal.example</code> traffic through to Traefik
					when available. All traffic flows through a single entry point on port 443 using <Abbr>SNI</Abbr>-based
					routing.
				</p>
			</Section>

			{/* Traffic Flow */}
			<Section title="How Traffic Flows">
				<div className="bg-gray-50 dark:bg-zinc-900/60 border border-gray-200/60 dark:border-zinc-800 rounded-lg p-4 space-y-2">
					<div className="text-[10px] font-medium uppercase tracking-wider text-gray-400 dark:text-zinc-500 mb-3">
						<Abbr>HTTPS</Abbr> Traffic (port 443)
					</div>
					<DiagramRow left="Browser :443" arrow="iptables NAT" right="SNI Router :9443" note="port redirect" />
					<DiagramRow left="SNI Router" arrow="*.lvh.me" right="Bun HTTPS :9444" note="local TLS" />
					<DiagramRow left="SNI Router" arrow="*.internal.example" right="Traefik :443" note="TCP passthrough" />
					<DiagramRow left="Bun HTTPS" right="Docker containers" note="reverse proxy" />

					<div className="text-[10px] font-medium uppercase tracking-wider text-gray-400 dark:text-zinc-500 mt-4 mb-3">
						<Abbr>HTTP</Abbr> Traffic (port 80)
					</div>
					<DiagramRow left="Browser :80" arrow="iptables NAT" right="Redirect :9080" note="301 to HTTPS" />
				</div>
			</Section>

			{/* SNI Router */}
			<Section title="SNI Router">
				<p>
					<Abbr>SNI</Abbr> (Server Name Indication) is a <Abbr>TLS</Abbr> extension that sends the target hostname in
					plaintext during the <Abbr>TLS</Abbr> handshake — before encryption begins. The <Abbr>SNI</Abbr> router
					inspects the first <Abbr>TLS</Abbr> ClientHello packet, extracts the hostname, and pipes the raw{" "}
					<Abbr>TCP</Abbr> connection to the right backend.
				</p>
				<p>
					This is called <strong>TCP passthrough</strong> — the router never decrypts traffic. This allows Traefik to
					keep using its own certificates while Bun uses <Abbr>mkcert</Abbr> certificates, all on the same port 443.
				</p>
			</Section>

			{/* TLS Certificates */}
			<Section title="TLS Certificates">
				<p>
					The Bun <Abbr>HTTPS</Abbr> server uses <Abbr>mkcert</Abbr> to generate locally-trusted wildcard certificates.
					Two certificate sets are configured via <Abbr>SNI</Abbr>:
				</p>
				<ul className="list-disc pl-4 space-y-1">
					<li>
						<code>*.lvh.me</code> — always handled by the Bun server
					</li>
					<li>
						<code>*.internal.example</code> — used as fallback when Traefik is not running
					</li>
				</ul>
				<p>
					<code>lvh.me</code> is a special domain that resolves all subdomains to <code>127.0.0.1</code> via public{" "}
					<Abbr>DNS</Abbr>, eliminating the need for <code>/etc/hosts</code> entries.
				</p>
			</Section>

			{/* Docker Discovery */}
			<Section title="Docker Service Discovery">
				<p>The proxy discovers containers on the shared Docker network using two label formats:</p>

				<div className="bg-gray-50 dark:bg-zinc-900/60 border border-gray-200/60 dark:border-zinc-800 rounded-lg p-4 space-y-3 mt-2">
					<div>
						<div className="text-[10px] font-medium uppercase tracking-wider text-sky-500 mb-1">
							Native labels (proxy.*)
						</div>
						<code className="text-[11px] block space-y-0.5">
							<div>proxy.host: "myapp.lvh.me"</div>
							<div>proxy.port: "3000"</div>
							<div>proxy.path: "/" (optional)</div>
							<div>proxy.strip: "true" (optional)</div>
						</code>
					</div>

					<div>
						<div className="text-[10px] font-medium uppercase tracking-wider text-orange-500 mb-1">
							Traefik labels (auto-parsed)
						</div>
						<code className="text-[11px] block space-y-0.5">
							<div>traefik.enable: "true"</div>
							<div>traefik.http.routers.app.rule: "Host(`app.internal.example`)"</div>
							<div>traefik.http.services.app.loadbalancer.server.port: "3000"</div>
						</code>
					</div>
				</div>

				<p>
					Native <code>proxy.*</code> labels take precedence. If a container has both, only the native route is
					registered.
				</p>
			</Section>

			{/* Path-Based Routing */}
			<Section title="Path-Based Routing">
				<p>
					By default, each container gets its own subdomain (e.g., <code>app.lvh.me</code>, <code>api.lvh.me</code>).
					For cases where multiple services must share the same origin — such as a frontend that calls its API on the
					same domain to avoid <Abbr>CORS</Abbr> — you can use path-based routing.
				</p>

				<div className="bg-gray-50 dark:bg-zinc-900/60 border border-gray-200/60 dark:border-zinc-800 rounded-lg p-4 space-y-3 mt-2">
					<div>
						<div className="text-[10px] font-medium uppercase tracking-wider text-sky-500 mb-1">
							Example: Frontend + API on same host
						</div>
						<code className="text-[11px] block space-y-0.5">
							<div># Frontend container</div>
							<div>proxy.host: "app.lvh.me"</div>
							<div>proxy.port: "3000"</div>
							<div>&nbsp;</div>
							<div># API container</div>
							<div>proxy.host: "app.lvh.me"</div>
							<div>proxy.path: "/api"</div>
							<div>proxy.strip: "true"</div>
							<div>proxy.port: "8080"</div>
						</code>
					</div>
				</div>

				<p>
					<code>proxy.path</code> matches requests by path prefix. <code>proxy.strip</code> removes the prefix before
					forwarding — so <code>/api/users</code> arrives at the backend as <code>/users</code>.
				</p>
				<p>
					In most cases, prefer separate subdomains (<code>app-api.lvh.me</code>) over path routing. Use path routing
					only when same-origin is required.
				</p>
			</Section>

			{/* Traefik Fallback */}
			<Section title="Traefik Fallback">
				<p>
					When Traefik is running, <code>*.internal.example</code> traffic passes through to it via <Abbr>TCP</Abbr>{" "}
					passthrough. Traefik handles <Abbr>TLS</Abbr> termination with its own certificates.
				</p>
				<p>
					When Traefik is <strong>not running</strong>, the <Abbr>SNI</Abbr> router falls back to the local Bun{" "}
					<Abbr>HTTPS</Abbr> server. Containers with Traefik labels are auto-discovered and routed directly using{" "}
					<Abbr>mkcert</Abbr> certificates. No configuration change needed — just stop Traefik and everything still
					works.
				</p>
			</Section>

			{/* Comparison */}
			<Section title="Traefik vs local-proxy">
				<div className="rounded-lg border border-gray-200/60 dark:border-zinc-800 overflow-hidden mt-2">
					<table className="w-full text-xs">
						<thead className="bg-gray-50 dark:bg-zinc-900">
							<tr>
								<th className="text-left px-4 py-2.5 font-semibold text-gray-500 dark:text-zinc-400 w-1/3" />
								<th className="text-left px-4 py-2.5 font-semibold text-orange-500">Traefik</th>
								<th className="text-left px-4 py-2.5 font-semibold text-indigo-500">local-proxy</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-gray-100 dark:divide-zinc-800/60">
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Runtime</td>
								<td className="px-4 py-2">Go binary in Docker container</td>
								<td className="px-4 py-2">Bun process on host</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Config</td>
								<td className="px-4 py-2">YAML files + Docker labels (verbose)</td>
								<td className="px-4 py-2">
									Simple <code>proxy.*</code> labels + <code>routes.yaml</code>
								</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Certificates</td>
								<td className="px-4 py-2">ACME / manual cert config</td>
								<td className="px-4 py-2">
									<Abbr>mkcert</Abbr> wildcard certs, auto-trusted
								</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Discovery</td>
								<td className="px-4 py-2">Docker labels only</td>
								<td className="px-4 py-2">Docker labels + static routes + Traefik label parsing</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Domains</td>
								<td className="px-4 py-2">
									<code>*.internal.example</code>
								</td>
								<td className="px-4 py-2">
									<code>*.lvh.me</code> (no hosts file needed)
								</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Dashboard</td>
								<td className="px-4 py-2">Built-in web UI</td>
								<td className="px-4 py-2">
									Custom React UI at <code>proxy.lvh.me</code>
								</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Startup</td>
								<td className="px-4 py-2">Docker container boot (~2-5s)</td>
								<td className="px-4 py-2">
									Instant (<code>bun run dev</code>)
								</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">WebSocket</td>
								<td className="px-4 py-2">Requires middleware config</td>
								<td className="px-4 py-2">
									Native support (Vite <Abbr>HMR</Abbr>, etc.)
								</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Use case</td>
								<td className="px-4 py-2">
									Work apps (<code>*.internal.example</code>)
								</td>
								<td className="px-4 py-2">
									Personal projects (<code>*.lvh.me</code>)
								</td>
							</tr>
						</tbody>
					</table>
				</div>
			</Section>
		</div>
	);
}
