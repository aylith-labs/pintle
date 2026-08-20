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

// ExpectedHost is a hostname this machine is supposed to serve. A host declared here
// with no matching route is a gap worth naming: absence of a route means the service
// behind it is down, not that it was never configured.
type ExpectedHost struct {
	Host    string `json:"host" yaml:"host"`
	Why     string `json:"why,omitempty" yaml:"why"`
	Project string `json:"project,omitempty" yaml:"project"`
}

type Message struct {
	ProviderName string
	Routes       []Route
	TcpRoutes    []TcpRoute
	Passthrough  []PassthroughDomain
	Expected     []ExpectedHost
}

type Provider interface {
	Run(ctx context.Context, configCh chan<- Message) error
}
