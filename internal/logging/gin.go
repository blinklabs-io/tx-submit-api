package logging

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// GinLogger returns a middleware that logs HTTP requests with slog.
func GinLogger(logger *slog.Logger, skipPaths []string) gin.HandlerFunc {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, path := range skipPaths {
		skip[path] = struct{}{}
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		if _, ok := skip[path]; ok {
			return
		}

		latency := time.Since(start)
		entry := []any{
			"status", c.Writer.Status(),
			"method", c.Request.Method,
			"path", path,
			"query", raw,
			"ip", c.ClientIP(),
			"userAgent", c.Request.UserAgent(),
			"latency", latency.String(),
			"size", c.Writer.Size(),
		}
		if len(c.Errors) > 0 {
			entry = append(entry, "errors", c.Errors.String())
		}

		logger.Info("request completed", entry...)
	}
}

// GinRecovery recovers from panics and logs them.
func GinRecovery(logger *slog.Logger, includeStack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				fields := []any{"err", err}
				if includeStack {
					fields = append(fields, "stack", string(debug.Stack()))
				}
				logger.Error("panic recovered", fields...)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
