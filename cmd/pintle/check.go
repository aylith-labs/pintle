package main

import (
	"fmt"

	"github.com/aylith-labs/pintle/internal/config"
	"github.com/aylith-labs/pintle/internal/provider/file"
)

func runCheck(args []string) error {
	cfg, err := config.LoadArgs(args)
	if err != nil {
		return err
	}
	msg, err := file.New(cfg.RoutesFile, cfg.HostAddress).Check()
	if err != nil {
		return err
	}
	if err := cfg.ValidateMessage(msg); err != nil {
		return err
	}
	fmt.Printf("Valid routes file %s: %d HTTP route(s), %d TCP route(s), %d passthrough domain(s)\n", cfg.RoutesFile, len(msg.Routes), len(msg.TcpRoutes), len(msg.Passthrough))
	return nil
}
