package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aylith-labs/pintle/internal/config"
)

// runDoctor answers "what is pintle, where is its config, and how do I restart it" for
// whoever is debugging — including when the proxy is down, which is exactly when the
// question gets asked and when the running instance cannot answer it.
func runDoctor() {
	cfg := config.Load()

	fmt.Printf("pintle %s\n\n", version)
	fmt.Println("Configuration (resolved locally, true whether or not pintle is running)")
	fmt.Printf("  routes file    %s\n", cfg.RoutesFile)
	fmt.Printf("  certs dir      %s\n", cfg.CertsDir)
	fmt.Printf("  base domain    %s\n", cfg.BaseDomain)
	fmt.Printf("  dashboard      https://%s\n", cfg.DashboardHost)
	fmt.Printf("  docker network %s\n", cfg.DockerNetwork)
	fmt.Printf("  label dialects pintle.* (native), traefik.*, caddy*\n")

	self, err := fetchSelf(cfg.DashboardHost)
	if err != nil {
		fmt.Printf("\nNot reachable at https://%s/api/self: %v\n", cfg.DashboardHost, err)
		fmt.Println("  pintle is down, or something else holds the port.")
		fmt.Println("  Start it:  docker compose up -d --build   (from the pintle checkout)")
		fmt.Println("  A `pgrep pintle` hit does NOT mean it is running host-native —")
		fmt.Println("  compose sets `pid: host`, so the containerised binary shows up as a host process.")
		os.Exit(1)
	}

	fmt.Println("\nRunning")
	fmt.Printf("  uptime         %ds\n", self.UptimeSec)
	fmt.Printf("  in docker      %v\n", self.InDocker)
	if self.Container.ContainerName != "" {
		fmt.Printf("  container      %s (compose project %s)\n", self.Container.ContainerName, self.Container.ComposeProject)
		fmt.Printf("  checkout       %s\n", self.Container.WorkingDir)
	}
	fmt.Printf("  certs loaded   %v\n", self.CertDomains)
	fmt.Printf("  restart with   %s\n", self.RestartCommand)
	if self.PidHostNote != "" {
		fmt.Printf("\n  note: %s\n", self.PidHostNote)
	}

	if len(self.Expected) == 0 {
		fmt.Println("\nNo expected hosts declared (add an `expect:` list to routes.yaml).")
		return
	}

	var missing int
	fmt.Println("\nExpected hosts")
	for _, exp := range self.Expected {
		mark := "ok "
		if !exp.Routed {
			mark = "MISSING"
			missing++
		}
		fmt.Printf("  %-8s %s\n", mark, exp.Host)
		if !exp.Routed && exp.Why != "" {
			fmt.Printf("           why it matters: %s\n", exp.Why)
		}
	}
	if missing > 0 {
		fmt.Printf("\n%d expected host(s) have no route. A missing route means the service behind it\n", missing)
		fmt.Println("is down — routes for label-discovered containers only exist while they run.")
		os.Exit(1)
	}
}

type selfReport struct {
	UptimeSec   int64    `json:"uptimeSec"`
	InDocker    bool     `json:"inDocker"`
	CertDomains []string `json:"certDomains"`
	PidHostNote string   `json:"pidHostNote"`
	Container   struct {
		ContainerName  string `json:"containerName"`
		ComposeProject string `json:"composeProject"`
		WorkingDir     string `json:"workingDir"`
	} `json:"container"`
	RestartCommand string `json:"restartCommand"`
	Expected       []struct {
		Host   string `json:"host"`
		Why    string `json:"why"`
		Routed bool   `json:"routed"`
	} `json:"expected"`
}

func fetchSelf(dashboardHost string) (*selfReport, error) {
	// The mkcert root is not necessarily trusted in this process's trust store, and the
	// question being answered is "is pintle up", not "is its cert valid".
	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := client.Get("https://" + dashboardHost + "/api/self")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var out selfReport
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
