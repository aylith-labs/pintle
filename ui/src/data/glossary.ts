export type GlossaryEntry = {
	term: string;
	description: string;
};

export const glossary: Record<string, GlossaryEntry> = {
	CORS: {
		term: 'Cross-Origin Resource Sharing',
		description:
			'A browser security mechanism that restricts web pages from making requests to a different origin (domain, port, or protocol) than the one that served the page.',
	},
	DNS: {
		term: 'Domain Name System',
		description:
			'Translates human-readable domain names (like lvh.me) into IP addresses that computers use to connect.',
	},
	HMR: {
		term: 'Hot Module Replacement',
		description:
			'A development feature that updates code in the browser without a full page reload, preserving application state.',
	},
	HTTP: {
		term: 'Hypertext Transfer Protocol',
		description:
			'The foundation protocol of the web. Defines how browsers request pages and servers respond with content.',
	},
	HTTPS: {
		term: 'HTTP Secure',
		description:
			'HTTP encrypted with TLS. All traffic between browser and server is encrypted, preventing eavesdropping.',
	},
	IP: {
		term: 'Internet Protocol',
		description: 'The network protocol that assigns addresses to devices and routes packets between them.',
	},
	iptables: {
		term: 'iptables',
		description:
			"A Linux kernel firewall tool that can redirect network traffic. Used here to forward ports 443/80 to the proxy's high ports.",
	},
	mkcert: {
		term: 'mkcert',
		description:
			'A tool that creates locally-trusted TLS certificates for development. No configuration needed — certificates are automatically trusted by the system.',
	},
	NAT: {
		term: 'Network Address Translation',
		description:
			'A method of remapping network addresses. Used by iptables to redirect traffic from standard ports to proxy ports.',
	},
	SNI: {
		term: 'Server Name Indication',
		description:
			'A TLS extension that sends the target hostname in plaintext during the handshake, allowing a single server to route traffic to different backends based on the requested domain.',
	},
	SSL: {
		term: 'Secure Sockets Layer',
		description: 'The predecessor to TLS. The term is still commonly used but modern connections use TLS.',
	},
	TCP: {
		term: 'Transmission Control Protocol',
		description:
			'A reliable transport protocol that ensures data arrives in order and without errors. HTTP and TLS run on top of TCP.',
	},
	TLS: {
		term: 'Transport Layer Security',
		description:
			'The cryptographic protocol that provides encrypted communication over a network. Successor to SSL, used for HTTPS connections.',
	},
};
