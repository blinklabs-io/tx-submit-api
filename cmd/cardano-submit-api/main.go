package main

import (
	"flag"
	"fmt"
	"github.com/cloudstruct/go-cardano-submit-api/internal/api"
	"github.com/cloudstruct/go-cardano-submit-api/internal/config"
	"github.com/cloudstruct/go-cardano-submit-api/internal/logging"
	"os"
)

var cmdlineFlags struct {
	configFile string
}

func main() {
	flag.StringVar(&cmdlineFlags.configFile, "config", "", "path to config file to load")
	flag.Parse()

	// Load config
	cfg, err := config.Load(cmdlineFlags.configFile)
	if err != nil {
		fmt.Printf("Failed to load config: %s\n", err)
		os.Exit(1)
	}

	// Configure logging
	logging.Setup(&cfg.Logging)
	logger := logging.GetLogger()
	// Sync logger on exit
	defer func() {
		if err := logger.Sync(); err != nil {
			// We don't actually care about the error here, but we have to do something
			// to appease the linter
			return
		}
	}()

	// Start API listener
	logger.Infof("starting API listener on %s:%d", cfg.Api.ListenAddress, cfg.Api.ListenPort)
	if err := api.Start(cfg); err != nil {
		logger.Fatalf("failed to start API: %s", err)
	}

	// Wait forever
	select {}
}
