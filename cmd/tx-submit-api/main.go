// Copyright 2025 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec G108
	"os"
	"time"

	"github.com/blinklabs-io/tx-submit-api/internal/api"
	"github.com/blinklabs-io/tx-submit-api/internal/config"
	"github.com/blinklabs-io/tx-submit-api/internal/logging"
	"github.com/blinklabs-io/tx-submit-api/internal/version"
	"go.uber.org/automaxprocs/maxprocs"
)

var cmdlineFlags struct {
	configFile string
}

func logPrintf(format string, v ...any) {
	logging.GetLogger().Info(fmt.Sprintf(format, v...))
}

func main() {
	flag.StringVar(
		&cmdlineFlags.configFile,
		"config",
		"",
		"path to config file to load",
	)
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

	logger.Info("starting tx-submit-api", "version", version.GetVersionString())

	// Configure max processes with our logger wrapper, toss undo func
	_, err = maxprocs.Set(maxprocs.Logger(logPrintf))
	if err != nil {
		// If we hit this, something really wrong happened
		logger.Error("maxprocs setup failed", "err", err)
		os.Exit(1)
	}

	// Start debug listener
	if cfg.Debug.ListenPort > 0 {
		logger.Info(
			"starting debug listener",
			"address", cfg.Debug.ListenAddress,
			"port", cfg.Debug.ListenPort,
		)
		go func() {
			debugger := &http.Server{
				Addr: fmt.Sprintf(
					"%s:%d",
					cfg.Debug.ListenAddress,
					cfg.Debug.ListenPort,
				),
				ReadHeaderTimeout: 60 * time.Second,
			}
			err := debugger.ListenAndServe()
			if err != nil {
				logger.Error("failed to start debug listener", "err", err)
				os.Exit(1)
			}
		}()
	}

	// Start API listener
	if err := api.Start(cfg); err != nil {
		logger.Error("failed to start API", "err", err)
		os.Exit(1)
	}

	// Wait forever
	select {}
}
