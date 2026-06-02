package provider

import "context"

type Route struct {
	Hostname      string `json:"hostname"`
	IsRegexp      bool   `json:"isRegexp,omitempty"` // Hostname is a regular expression (Traefik HostRegexp)
	Path          string `json:"path"`
	Target        string `json:"target"`
	StripPath     bool   `json:"stripPath"`
	H2C           bool   `json:"h2c,omitempty"` // upstream speaks HTTP/2 cleartext (gRPC)
	Source        string `json:"source"`        // "docker", "static", "traefik", "caddy"
	ContainerName string `json:"containerName,omitempty"`
}

type TcpRoute struct {
	Hostname      string `json:"hostname"`
	TargetHost    string `json:"targetHost"`
	TargetPort    int    `json:"targetPort"`
	ListenPort    int    `json:"listenPort"`
	Source        string `json:"source"` // "docker", "static", "traefik"
	ContainerName string `json:"containerName,omitempty"`
}

type PassthroughDomain struct {
	Domain string `json:"domain" yaml:"domain"`
	Target string `json:"target" yaml:"target"`
}

type Message struct {
	ProviderName string
	Routes       []Route
	TcpRoutes    []TcpRoute
	Passthrough  []PassthroughDomain
}

type Provider interface {
	Run(ctx context.Context, configCh chan<- Message) error
}
