package docker

import (
	"regexp"
	"strconv"
	"strings"
)

type parsedLabels struct {
	Hosts []string
	Port  int
	Path  string
	Strip bool
}

// parseProxyLabels parses local-proxy.* labels.
func parseProxyLabels(labels map[string]string) *parsedLabels {
	hostLabel, ok := labels["local-proxy.host"]
	if !ok {
		return nil
	}

	hosts := strings.Split(hostLabel, ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}

	port := 0
	if portStr, ok := labels["local-proxy.port"]; ok {
		port, _ = strconv.Atoi(portStr)
	}

	path := labels["local-proxy.path"]
	if path == "" {
		path = "/"
	}

	strip := labels["local-proxy.strip"] == "true"

	return &parsedLabels{
		Hosts: hosts,
		Port:  port,
		Path:  path,
		Strip: strip,
	}
}

var (
	traefikRouterRuleRe  = regexp.MustCompile(`^traefik\.http\.routers\..+\.rule$`)
	traefikServicePortRe = regexp.MustCompile(`^traefik\.http\.services\..+\.loadbalancer\.server\.port$`)
	traefikStripPrefixRe = regexp.MustCompile(`^traefik\.http\.middlewares\..+\.stripprefix\.prefixes$`)
	hostRe               = regexp.MustCompile(`Host\(` + "`" + `([^` + "`" + `]+)` + "`" + `\)`)
	pathPrefixRe         = regexp.MustCompile(`PathPrefix\(` + "`" + `([^` + "`" + `]+)` + "`" + `\)`)
)

// parseTraefikLabels parses traefik.http.* labels.
func parseTraefikLabels(labels map[string]string) *parsedLabels {
	if labels["traefik.enable"] != "true" {
		return nil
	}

	// Find router rule
	var ruleValue string
	for key, value := range labels {
		if traefikRouterRuleRe.MatchString(key) {
			ruleValue = value
			break
		}
	}
	if ruleValue == "" {
		return nil
	}

	// Parse Host(`...`)
	hostMatches := hostRe.FindAllStringSubmatch(ruleValue, -1)
	if len(hostMatches) == 0 {
		return nil
	}
	var hosts []string
	for _, m := range hostMatches {
		hosts = append(hosts, m[1])
	}

	// Parse optional PathPrefix(`...`)
	path := "/"
	if pathMatch := pathPrefixRe.FindStringSubmatch(ruleValue); pathMatch != nil {
		path = pathMatch[1]
	}

	// Find port
	port := 0
	for key, value := range labels {
		if traefikServicePortRe.MatchString(key) {
			port, _ = strconv.Atoi(value)
			break
		}
	}

	// Check for stripprefix middleware
	strip := false
	for key := range labels {
		if traefikStripPrefixRe.MatchString(key) {
			strip = true
			break
		}
	}

	return &parsedLabels{
		Hosts: hosts,
		Port:  port,
		Path:  path,
		Strip: strip,
	}
}

var caddyPrefixRe = regexp.MustCompile(`^(caddy(?:_\d+)?)$`)
var caddyUpstreamsRe = regexp.MustCompile(`\{\{upstreams(?:\s+https?)?\s+(\d+)\}\}`)

// parseCaddyLabels parses caddy/caddy_N labels. Returns multiple configs for multi-domain.
func parseCaddyLabels(labels map[string]string) []parsedLabels {
	type caddyConfig struct {
		host  string
		port  int
		path  string
		strip bool
	}

	configs := make(map[string]*caddyConfig)

	// Find all caddy prefixes
	for key, value := range labels {
		if caddyPrefixRe.MatchString(key) {
			configs[key] = &caddyConfig{host: value, path: "/"}
		}
	}

	if len(configs) == 0 {
		return nil
	}

	// Parse directives for each prefix
	for key, value := range labels {
		for prefix, config := range configs {
			// Port from reverse_proxy
			if key == prefix+".reverse_proxy" {
				if m := caddyUpstreamsRe.FindStringSubmatch(value); m != nil {
					config.port, _ = strconv.Atoi(m[1])
				}
			}
			// Path with strip: handle_path
			if key == prefix+".handle_path" {
				config.path = strings.TrimRight(strings.TrimSuffix(value, "*"), "/")
				if config.path == "" {
					config.path = "/"
				}
				config.strip = true
			}
			// Path without strip: handle
			if key == prefix+".handle" && config.path == "/" {
				config.path = strings.TrimRight(strings.TrimSuffix(value, "*"), "/")
				if config.path == "" {
					config.path = "/"
				}
			}
		}
	}

	var results []parsedLabels
	for _, config := range configs {
		if config.host == "" {
			continue
		}
		results = append(results, parsedLabels{
			Hosts: []string{config.host},
			Port:  config.port,
			Path:  config.path,
			Strip: config.strip,
		})
	}

	return results
}

var (
	traefikTcpRouterRuleRe  = regexp.MustCompile(`^traefik\.tcp\.routers\..+\.rule$`)
	traefikTcpEntrypointsRe = regexp.MustCompile(`^traefik\.tcp\.routers\..+\.entrypoints$`)
	traefikTcpServicePortRe = regexp.MustCompile(`^traefik\.tcp\.services\..+\.loadbalancer\.server\.port$`)
	hostSNIRe               = regexp.MustCompile(`HostSNI\(` + "`" + `([^` + "`" + `]+)` + "`" + `\)`)
)

type parsedTcpLabels struct {
	Hostname      string
	Entrypoint    string
	ContainerPort int
}

// parseTraefikTcpLabels parses traefik.tcp.* labels.
func parseTraefikTcpLabels(labels map[string]string) *parsedTcpLabels {
	if labels["traefik.enable"] != "true" {
		return nil
	}

	var hostname, entrypoint string

	for key, value := range labels {
		if traefikTcpRouterRuleRe.MatchString(key) {
			if m := hostSNIRe.FindStringSubmatch(value); m != nil {
				hostname = m[1]
			}
		}
		if traefikTcpEntrypointsRe.MatchString(key) {
			entrypoint = value
		}
	}

	if hostname == "" || entrypoint == "" {
		return nil
	}

	containerPort := 0
	for key, value := range labels {
		if traefikTcpServicePortRe.MatchString(key) {
			containerPort, _ = strconv.Atoi(value)
			break
		}
	}
	if containerPort == 0 {
		return nil
	}

	return &parsedTcpLabels{
		Hostname:      hostname,
		Entrypoint:    entrypoint,
		ContainerPort: containerPort,
	}
}
