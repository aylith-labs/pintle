import { ArrowRight, Cube, FlowArrow, Terminal } from '@phosphor-icons/react';
import { useState } from 'react';
import { Abbr } from '../components/Abbr';
import type { RuntimeMode } from '../types';

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
		<div className="flex items-center gap-2 font-mono text-[0.6875rem]">
			<span className="text-gray-900 dark:text-zinc-100 min-w-[120px]">{left}</span>
			<ArrowRight size={12} className="text-indigo-400 shrink-0" />
			{arrow && <span className="text-indigo-400 text-[0.625rem]">{arrow}</span>}
			{arrow && <ArrowRight size={12} className="text-indigo-400 shrink-0" />}
			<span className="text-gray-900 dark:text-zinc-100">{right}</span>
			{note && <span className="text-gray-400 dark:text-zinc-500 ml-2">({note})</span>}
		</div>
	);
}

function ModeBadge({ mode }: { mode: RuntimeMode }) {
	const Icon = mode === 'docker' ? Cube : Terminal;
	const label = mode === 'docker' ? 'Docker' : 'Host-native';
	const color =
		mode === 'docker'
			? 'bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-200/60 dark:border-sky-800/40'
			: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-200/60 dark:border-amber-800/40';
	return (
		<span
			className={`inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded border text-[0.5625rem] font-medium ${color}`}
		>
			<Icon size={10} weight="bold" />
			{label}
		</span>
	);
}

function ModeToggle({
	mode,
	onChange,
	detectedMode,
}: { mode: RuntimeMode; onChange: (m: RuntimeMode) => void; detectedMode: RuntimeMode | null }) {
	return (
		<div className="flex items-center gap-2">
			{detectedMode && (
				<span className="text-[0.625rem] text-gray-400 dark:text-zinc-500 mr-1">
					detected: {detectedMode === 'docker' ? 'Docker' : 'host-native'}
				</span>
			)}
			<div className="flex rounded-md border border-gray-200/60 dark:border-zinc-700 overflow-hidden">
				<button
					type="button"
					onClick={() => onChange('docker')}
					className={`flex items-center gap-1 px-2.5 py-1 text-[0.625rem] font-medium transition-colors ${
						mode === 'docker'
							? 'bg-indigo-500/10 text-indigo-600 dark:text-indigo-400'
							: 'text-gray-500 dark:text-zinc-400 hover:bg-gray-50 dark:hover:bg-zinc-800'
					}`}
				>
					<Cube size={11} />
					Docker
				</button>
				<button
					type="button"
					onClick={() => onChange('host-native')}
					className={`flex items-center gap-1 px-2.5 py-1 text-[0.625rem] font-medium transition-colors border-l border-gray-200/60 dark:border-zinc-700 ${
						mode === 'host-native'
							? 'bg-indigo-500/10 text-indigo-600 dark:text-indigo-400'
							: 'text-gray-500 dark:text-zinc-400 hover:bg-gray-50 dark:hover:bg-zinc-800'
					}`}
				>
					<Terminal size={11} />
					Host-native
				</button>
			</div>
		</div>
	);
}

type ArchitecturePageProps = {
	mode: RuntimeMode | null;
};

export function ArchitecturePage({ mode: detectedMode }: ArchitecturePageProps) {
	const [mode, setMode] = useState<RuntimeMode>(detectedMode ?? 'docker');
	const isDocker = mode === 'docker';

	return (
		<div className="space-y-8 max-w-3xl">
			{/* Header with link back */}
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-2">
					<FlowArrow size={18} weight="bold" className="text-indigo-500" />
					<h1 className="text-sm font-semibold tracking-tight">Architecture</h1>
				</div>
				<div className="flex items-center gap-4">
					<ModeToggle mode={mode} onChange={setMode} detectedMode={detectedMode} />
					<a
						href="#/"
						className="text-[0.6875rem] font-medium text-indigo-500 hover:text-indigo-400 transition-colors flex items-center gap-1"
					>
						View Live Dashboard
						<ArrowRight size={12} />
					</a>
				</div>
			</div>

			{/* Overview */}
			<Section title="Overview">
				<p>
					local-proxy is a <Abbr>HTTPS</Abbr> reverse proxy for local development. It routes <code>*.lvh.me</code>{' '}
					domains through a Go-based server and passes <code>*.example-local.com</code> traffic through to Traefik
					when available. All traffic flows through a single entry point on port 443 using <Abbr>SNI</Abbr>-based
					routing.
				</p>
			</Section>

			{/* Traffic Flow */}
			<Section title="How Traffic Flows">
				<div className="bg-gray-50 dark:bg-zinc-900/60 border border-gray-200/60 dark:border-zinc-800 rounded-lg p-4 space-y-2">
					<div className="flex items-center gap-2 text-[0.625rem] font-medium uppercase tracking-wider text-gray-400 dark:text-zinc-500 mb-3">
						<Abbr>HTTPS</Abbr> Traffic (port 443) <ModeBadge mode={mode} />
					</div>
					{isDocker ? (
						<DiagramRow left="Browser :443" arrow="Docker port map" right="SNI Router :9443" note="443→9443" />
					) : (
						<DiagramRow left="Browser :443" arrow="iptables NAT" right="SNI Router :9443" note="port redirect" />
					)}
					<DiagramRow left="SNI Router" arrow="*.lvh.me" right="HTTPS Server :9444" note="local TLS" />
					<DiagramRow left="SNI Router" arrow="passthrough domains" right="Traefik :443" note="TCP passthrough" />
					<DiagramRow left="HTTPS Server" right="Docker containers" note="reverse proxy" />

					<div className="flex items-center gap-2 text-[0.625rem] font-medium uppercase tracking-wider text-gray-400 dark:text-zinc-500 mt-4 mb-3">
						<Abbr>HTTP</Abbr> Traffic (port 80) <ModeBadge mode={mode} />
					</div>
					{isDocker ? (
						<DiagramRow left="Browser :80" arrow="Docker port map" right="Redirect :9080" note="80→9080, 301 to HTTPS" />
					) : (
						<DiagramRow left="Browser :80" arrow="iptables NAT" right="Redirect :9080" note="301 to HTTPS" />
					)}

					<div className="mt-3">
						{isDocker ? (
							<p className="text-[0.625rem] text-gray-400 dark:text-zinc-500">
								<strong>Windows <Abbr>WSL</Abbr> note:</strong> You may also need{' '}
								<code>netsh interface portproxy</code> to forward from Windows localhost to the <Abbr>WSL</Abbr> IP.
							</p>
						) : (
							<p className="text-[0.625rem] text-gray-400 dark:text-zinc-500">
								Port redirection is managed by <code>scripts/start.sh</code> (iptables on Linux, pfctl on macOS).
								Run <code>scripts/stop.sh</code> to remove the rules.
							</p>
						)}
					</div>
				</div>
			</Section>

			{/* SNI Router */}
			<Section title="SNI Router">
				<p>
					<Abbr>SNI</Abbr> (Server Name Indication) is a <Abbr>TLS</Abbr> extension that sends the target hostname in
					plaintext during the <Abbr>TLS</Abbr> handshake — before encryption begins. The <Abbr>SNI</Abbr> router
					inspects the first <Abbr>TLS</Abbr> ClientHello packet, extracts the hostname, and pipes the raw{' '}
					<Abbr>TCP</Abbr> connection to the right backend.
				</p>
				<p>
					This is called <strong>TCP passthrough</strong> — the router never decrypts traffic. This allows Traefik to
					keep using its own certificates while local-proxy uses <Abbr>mkcert</Abbr> certificates, all on the same port 443.
				</p>
			</Section>

			{/* TLS Certificates */}
			<Section title="TLS Certificates">
				<p>
					The <Abbr>HTTPS</Abbr> server uses <Abbr>mkcert</Abbr> to generate locally-trusted wildcard certificates.
					Two certificate sets are configured via <Abbr>SNI</Abbr>:
				</p>
				<ul className="list-disc pl-4 space-y-1">
					<li>
						<code>*.lvh.me</code> — always handled by the HTTPS server
					</li>
					<li>
						<code>*.example-local.com</code> — used as fallback when Traefik is not running
					</li>
				</ul>
				<p>
					<code>lvh.me</code> is a special domain that resolves all subdomains to <code>127.0.0.1</code> via public{' '}
					<Abbr>DNS</Abbr>, eliminating the need for <code>/etc/hosts</code> entries.
				</p>
			</Section>

			{/* Docker Discovery */}
			<Section title="Docker Service Discovery">
				<p>The proxy discovers containers on the shared Docker network using two label formats:</p>

				<div className="bg-gray-50 dark:bg-zinc-900/60 border border-gray-200/60 dark:border-zinc-800 rounded-lg p-4 space-y-3 mt-2">
					<div>
						<div className="text-[0.625rem] font-medium uppercase tracking-wider text-sky-500 mb-1">
							Native labels (local-proxy.*)
						</div>
						<code className="text-[0.6875rem] block space-y-0.5">
							<div>local-proxy.host: "myapp.lvh.me"</div>
							<div>local-proxy.port: "3000"</div>
							<div>local-proxy.path: "/" (optional)</div>
							<div>local-proxy.strip: "true" (optional)</div>
						</code>
					</div>

					<div>
						<div className="text-[0.625rem] font-medium uppercase tracking-wider text-orange-500 mb-1">
							Traefik labels (auto-parsed)
						</div>
						<code className="text-[0.6875rem] block space-y-0.5">
							<div>traefik.enable: "true"</div>
							<div>traefik.http.routers.app.rule: "Host(`app.example-local.com`)"</div>
							<div>traefik.http.services.app.loadbalancer.server.port: "3000"</div>
						</code>
					</div>
				</div>

				<p>
					Native <code>local-proxy.*</code> labels take precedence. If a container has both, only the native route is
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
						<div className="text-[0.625rem] font-medium uppercase tracking-wider text-sky-500 mb-1">
							Example: Frontend + API on same host
						</div>
						<code className="text-[0.6875rem] block space-y-0.5">
							<div># Frontend container</div>
							<div>local-proxy.host: "app.lvh.me"</div>
							<div>local-proxy.port: "3000"</div>
							<div>&nbsp;</div>
							<div># API container</div>
							<div>local-proxy.host: "app.lvh.me"</div>
							<div>local-proxy.path: "/api"</div>
							<div>local-proxy.strip: "true"</div>
							<div>local-proxy.port: "8080"</div>
						</code>
					</div>
				</div>

				<p>
					<code>local-proxy.path</code> matches requests by path prefix. <code>local-proxy.strip</code> removes the prefix before
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
					When Traefik is running, <code>*.example-local.com</code> traffic passes through to it via <Abbr>TCP</Abbr>{' '}
					passthrough. Traefik handles <Abbr>TLS</Abbr> termination with its own certificates.
				</p>
				<p>
					When Traefik is <strong>not running</strong>, the <Abbr>SNI</Abbr> router falls back to the local{' '}
					<Abbr>HTTPS</Abbr> server. Containers with Traefik labels are auto-discovered and routed directly using{' '}
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
								<td className="px-4 py-2">
									<span className="inline-flex items-center gap-1.5">
										<ModeBadge mode={mode} />
										{isDocker ? 'Go binary in Docker (10 MB image)' : 'Go binary on host (~9 MB)'}
									</span>
								</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Config</td>
								<td className="px-4 py-2">YAML files + Docker labels (verbose)</td>
								<td className="px-4 py-2">
									Simple <code>local-proxy.*</code> labels + <code>routes.yaml</code>
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
									<code>*.example-local.com</code>
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
									<span className="inline-flex items-center gap-1.5">
										<ModeBadge mode={mode} />
										<code>{isDocker ? 'docker compose up -d' : './local-proxy --port-redirect'}</code>
									</span>
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
									Passthrough apps (<code>*.example-local.com</code>)
								</td>
								<td className="px-4 py-2">
									Personal projects (<code>*.lvh.me</code>)
								</td>
							</tr>
						</tbody>
					</table>
				</div>
			</Section>

			{/* Why Go */}
			<Section title="Why Go">
				<p>
					local-proxy was originally written in Bun (TypeScript) but rewritten in Go to fix fundamental reliability
					issues. Go is uniquely suited for reverse proxies:
				</p>

				<div className="bg-gray-50 dark:bg-zinc-900/60 border border-gray-200/60 dark:border-zinc-800 rounded-lg p-4 space-y-3 mt-2">
					<div>
						<div className="text-[0.625rem] font-medium uppercase tracking-wider text-emerald-500 mb-1">
							Concurrency
						</div>
						<p>
							Go spawns a goroutine per connection (~2 KB stack). Node/Bun/Deno use a single-threaded event loop —
							fine for <Abbr>I/O</Abbr>, but a reverse proxy manages many concurrent <Abbr>TCP</Abbr> connections with
							bidirectional data flow. <Abbr>SNI</Abbr> routing, WebSocket proxying, and <Abbr>TCP</Abbr> service routing
							all do raw socket piping that maps naturally to goroutines.
						</p>
					</div>

					<div>
						<div className="text-[0.625rem] font-medium uppercase tracking-wider text-emerald-500 mb-1">
							Connection pooling
						</div>
						<p>
							Go's <code>http.Transport</code> has battle-tested connection pooling (used by every Go <Abbr>HTTP</Abbr>{' '}
							client since 2012). Bun's <code>fetch()</code> has known socket reuse bugs that cause stalls at ~50
							concurrent requests. The Go proxy handles 260 req/s where Bun stalled and timed out.
						</p>
					</div>

					<div>
						<div className="text-[0.625rem] font-medium uppercase tracking-wider text-emerald-500 mb-1">
							Static binary
						</div>
						<p>
							<code>CGO_ENABLED=0 go build</code> produces a single ~9 MB file with zero runtime dependencies. Docker
							image is 10 MB (<code>FROM scratch</code>). Node/Bun/Deno need a ~100-200 MB runtime plus{' '}
							<code>node_modules</code>.
						</p>
					</div>

					<div>
						<div className="text-[0.625rem] font-medium uppercase tracking-wider text-emerald-500 mb-1">
							Standard library
						</div>
						<p>
							Go's stdlib includes a production-grade reverse proxy (<code>httputil.ReverseProxy</code>), <Abbr>TLS</Abbr>{' '}
							server with dynamic certificate selection, and raw <Abbr>TCP</Abbr> listeners. In Node/Bun these require
							third-party packages or manual implementation.
						</p>
					</div>

					<div>
						<div className="text-[0.625rem] font-medium uppercase tracking-wider text-emerald-500 mb-1">
							Embedded dashboard
						</div>
						<p>
							<code>{'//go:embed'}</code> compiles the React UI into the binary. No separate Vite container, no static
							file serving setup — the dashboard just works out of the binary.
						</p>
					</div>
				</div>

				<div className="rounded-lg border border-gray-200/60 dark:border-zinc-800 overflow-hidden mt-4">
					<table className="w-full text-xs">
						<thead className="bg-gray-50 dark:bg-zinc-900">
							<tr>
								<th className="text-left px-4 py-2.5 font-semibold text-gray-500 dark:text-zinc-400 w-1/3" />
								<th className="text-left px-4 py-2.5 font-semibold text-indigo-500">Go</th>
								<th className="text-left px-4 py-2.5 font-semibold text-amber-500">Node / Bun / Deno</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-gray-100 dark:divide-zinc-800/60">
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Concurrency</td>
								<td className="px-4 py-2">Goroutine per connection (~2 KB)</td>
								<td className="px-4 py-2">Single-threaded event loop</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Proxy throughput</td>
								<td className="px-4 py-2 text-emerald-600 dark:text-emerald-400">260 req/s, 0 failures</td>
								<td className="px-4 py-2 text-red-500">Stalls after ~50 requests</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Page load overhead</td>
								<td className="px-4 py-2">+85ms / 7% (250 resources)</td>
								<td className="px-4 py-2">N/A (stalls under load)</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Binary size</td>
								<td className="px-4 py-2">~9 MB (static, no deps)</td>
								<td className="px-4 py-2">~100-200 MB (runtime + node_modules)</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Docker image</td>
								<td className="px-4 py-2">10 MB (FROM scratch)</td>
								<td className="px-4 py-2">~200 MB + separate UI container</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Reverse proxy</td>
								<td className="px-4 py-2">stdlib <code>httputil.ReverseProxy</code></td>
								<td className="px-4 py-2">Manual or third-party</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">TLS / SNI</td>
								<td className="px-4 py-2">stdlib <code>crypto/tls</code></td>
								<td className="px-4 py-2">stdlib (adequate)</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Docker SDK</td>
								<td className="px-4 py-2">First-party (Docker is Go)</td>
								<td className="px-4 py-2">Community wrapper (dockerode)</td>
							</tr>
							<tr>
								<td className="px-4 py-2 font-medium text-gray-900 dark:text-zinc-200">Embed assets</td>
								<td className="px-4 py-2"><code>{'//go:embed'}</code> (built-in)</td>
								<td className="px-4 py-2">Requires bundler or separate server</td>
							</tr>
						</tbody>
					</table>
				</div>
			</Section>
		</div>
	);
}
