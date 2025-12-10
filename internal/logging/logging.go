// Copyright 2023 Blink Labs Software
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

package logging

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/blinklabs-io/tx-submit-api/internal/config"
)

var (
	globalLogger *slog.Logger
	accessLogger *slog.Logger
)

func Setup(cfg *config.LoggingConfig) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		log.Fatalf("error configuring logger: %s", err)
	}

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				attr.Key = "timestamp"
				attr.Value = slog.StringValue(
					attr.Value.Time().Format(time.RFC3339),
				)
			}
			return attr
		},
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	globalLogger = slog.New(handler)
	accessLogger = globalLogger.With(slog.String("type", "access"))
}

func GetLogger() *slog.Logger {
	return globalLogger
}

func GetAccessLogger() *slog.Logger {
	return accessLogger
}

func parseLevel(level string) (slog.Leveler, error) {
	if level == "" {
		return slog.LevelInfo, nil
	}
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return nil, fmt.Errorf("invalid log level: %s", level)
	}
}
