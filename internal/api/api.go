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

package api

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"time"

	ouroboros "github.com/blinklabs-io/gouroboros"
	"github.com/blinklabs-io/gouroboros/protocol/localtxsubmission"
	"github.com/fxamacker/cbor/v2"
	cors "github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware

	_ "github.com/blinklabs-io/tx-submit-api/docs" // docs is generated by Swag CLI
	"github.com/blinklabs-io/tx-submit-api/internal/config"
	"github.com/blinklabs-io/tx-submit-api/internal/logging"
	"github.com/blinklabs-io/tx-submit-api/submit"
)

//go:embed static
var staticFS embed.FS

// @title			tx-submit-api
// @version		v0
// @description	Cardano Transaction Submit API
// @BasePath		/
// @contact.name	Blink Labs Software
// @contact.url	https://blinklabs.io
// @contact.email	support@blinklabs.io
//
// @license.name	Apache 2.0
// @license.url	http://www.apache.org/licenses/LICENSE-2.0.html
func Start(cfg *config.Config) error {
	// Standard logging
	logger := logging.GetLogger()
	if cfg.Tls.CertFilePath != "" && cfg.Tls.KeyFilePath != "" {
		logger.Infof(
			"starting API TLS listener on %s:%d",
			cfg.Api.ListenAddress,
			cfg.Api.ListenPort,
		)
	} else {
		logger.Infof(
			"starting API listener on %s:%d",
			cfg.Api.ListenAddress,
			cfg.Api.ListenPort,
		)
	}
	// Disable gin debug and color output
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()

	// Configure API router
	router := gin.New()
	// Catch panics and return a 500
	router.Use(gin.Recovery())
	// Configure CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = []string{"hx-current-url","hx-request","hx-target","hx-trigger"}
	router.Use(cors.New(corsConfig))
	// Access logging
	accessLogger := logging.GetAccessLogger()
	skipPaths := []string{}
	if cfg.Logging.Healthchecks {
		skipPaths = append(skipPaths, "/healthcheck")
		logger.Infof("disabling access logs for /healthcheck")
	}
	router.Use(ginzap.GinzapWithConfig(accessLogger, &ginzap.Config{
		TimeFormat: time.RFC3339,
		UTC:        true,
		SkipPaths:  skipPaths,
	}))
	router.Use(ginzap.RecoveryWithZap(accessLogger, true))

	// Configure static route
	fsys, err := fs.Sub(staticFS, "static")
	if err != nil {
		return err
	}
	router.StaticFS("/ui", http.FS(fsys))
	// Redirect from root
	router.GET("/", func(c *gin.Context) {
		c.Request.URL.Path = "/ui"
		router.HandleContext(c)
	})

	// Create a healthcheck (before metrics so it's not instrumented)
	router.GET("/healthcheck", handleHealthcheck)
	// Create a swagger endpoint (not instrumented)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Metrics
	metricsRouter := gin.New()
	// Configure CORS
	metricsRouter.Use(cors.New(corsConfig))
	metrics := ginmetrics.GetMonitor()
	// Set metrics path
	metrics.SetMetricPath("/")
	// Set metrics router
	metrics.Expose(metricsRouter)
	// Use metrics middleware without exposing path in main app router
	metrics.UseWithoutExposingEndpoint(router)

	// Custom metrics
	failureMetric := &ginmetrics.Metric{
		// This is a Gauge because input-output-hk's is a gauge
		Type:        ginmetrics.Gauge,
		Name:        "tx_submit_fail_count",
		Description: "transactions failed",
		Labels:      nil,
	}
	submittedMetric := &ginmetrics.Metric{
		// This is a Gauge because input-output-hk's is a gauge
		Type:        ginmetrics.Gauge,
		Name:        "tx_submit_count",
		Description: "transactions submitted",
		Labels:      nil,
	}
	// Add to global monitor object
	_ = ginmetrics.GetMonitor().AddMetric(failureMetric)
	_ = ginmetrics.GetMonitor().AddMetric(submittedMetric)
	// Initialize metrics
	_ = ginmetrics.GetMonitor().GetMetric("tx_submit_fail_count").SetGaugeValue(nil, 0.0)
	_ = ginmetrics.GetMonitor().GetMetric("tx_submit_count").SetGaugeValue(nil, 0.0)

	// Start metrics listener
	go func() {
		// TODO: return error if we cannot initialize metrics
		logger.Infof("starting metrics listener on %s:%d",
			cfg.Metrics.ListenAddress,
			cfg.Metrics.ListenPort)
		_ = metricsRouter.Run(fmt.Sprintf("%s:%d",
			cfg.Metrics.ListenAddress,
			cfg.Metrics.ListenPort))
	}()

	// Configure API routes
	router.POST("/api/submit/tx", handleSubmitTx)
	router.GET("/api/hastx/:tx_hash", handleHasTx)

	// Start API listener
	if cfg.Tls.CertFilePath != "" && cfg.Tls.KeyFilePath != "" {
		return router.RunTLS(
			fmt.Sprintf("%s:%d", cfg.Api.ListenAddress, cfg.Api.ListenPort),
			cfg.Tls.CertFilePath,
			cfg.Tls.KeyFilePath,
		)
	} else {
		return router.Run(fmt.Sprintf("%s:%d",
			cfg.Api.ListenAddress,
			cfg.Api.ListenPort))
	}
}

func handleHealthcheck(c *gin.Context) {
	// TODO: add some actual health checking here
	c.JSON(200, gin.H{"failed": false})
}

// Path parameters for GET requests
type TxHashPathParams struct {
	TxHash string `uri:"tx_hash" binding:"required"` // Transaction hash
}

// handleHasTx godoc
//
//	@Summary		HasTx
//	@Description	Determine if a given transaction ID exists in the node mempool.
//	@Produce		json
//	@Param			tx_hash	path		string	true	"Transaction Hash"
//	@Success		200		{object}	string	"Ok"
//	@Failure		400		{object}	string	"Bad Request"
//	@Failure		404		{object}	string	"Not Found"
//	@Failure		415		{object}	string	"Unsupported Media Type"
//	@Failure		500		{object}	string	"Server Error"
//	@Router			/api/hastx/{tx_hash} [get]
func handleHasTx(c *gin.Context) {
	// First, initialize our configuration and loggers
	cfg := config.GetConfig()
	logger := logging.GetLogger()

	var uriParams TxHashPathParams
	if err := c.ShouldBindUri(&uriParams); err != nil {
		logger.Errorf("failed to bind transaction hash from path: %s", err)
		c.JSON(400, fmt.Sprintf("invalid transaction hash: %s", err))
		return
	}

	txHash := uriParams.TxHash
	// convert to cbor bytes
	cborData, err := cbor.Marshal(txHash)
	if err != nil {
		logger.Errorf("failed to encode transaction hash to CBOR: %s", err)
		c.JSON(
			400,
			fmt.Sprintf("failed to encode transaction hash to CBOR: %s", err),
		)
		return
	}

	// Connect to cardano-node and check for transaction
	errorChan := make(chan error)
	oConn, err := ouroboros.NewConnection(
		ouroboros.WithNetworkMagic(uint32(cfg.Node.NetworkMagic)),
		ouroboros.WithErrorChan(errorChan),
		ouroboros.WithNodeToNode(false),
	)
	if err != nil {
		logger.Errorf("failure creating Ouroboros connection: %s", err)
		c.JSON(500, "failure communicating with node")
		return
	}
	if cfg.Node.Address != "" && cfg.Node.Port > 0 {
		if err := oConn.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Node.Address, cfg.Node.Port)); err != nil {
			logger.Errorf("failure connecting to node via TCP: %s", err)
			c.JSON(500, "failure communicating with node")
			return
		}
	} else {
		if err := oConn.Dial("unix", cfg.Node.SocketPath); err != nil {
			logger.Errorf("failure connecting to node via UNIX socket: %s", err)
			c.JSON(500, "failure communicating with node")
			return
		}
	}
	// Start async error handler
	go func() {
		err, ok := <-errorChan
		if ok {
			logger.Errorf("failure communicating with node: %s", err)
			c.JSON(500, "failure communicating with node")
		}
	}()
	defer func() {
		// Close Ouroboros connection
		oConn.Close()
	}()
	hasTx, err := oConn.LocalTxMonitor().Client.HasTx(cborData)
	if err != nil {
		logger.Errorf("failure getting transaction: %s", err)
		c.JSON(500, fmt.Sprintf("failure getting transaction: %s", err))
	}
	if !hasTx {
		c.JSON(404, "transaction not found in mempool")
		return
	}
	c.JSON(200, "transaction found in mempool")
}

// handleSubmitTx godoc
//
//	@Summary		Submit Tx
//	@Description	Submit an already serialized transaction to the network.
//	@Produce		json
//	@Param			Content-Type	header		string	true	"Content type"	Enums(application/cbor)
//	@Success		202				{object}	string	"Ok"
//	@Failure		400				{object}	string	"Bad Request"
//	@Failure		415				{object}	string	"Unsupported Media Type"
//	@Failure		500				{object}	string	"Server Error"
//	@Router			/api/submit/tx [post]
func handleSubmitTx(c *gin.Context) {
	// First, initialize our configuration and loggers
	cfg := config.GetConfig()
	logger := logging.GetLogger()
	// Check our headers for content-type
	if c.ContentType() != "application/cbor" {
		// Log the error, return an error to the user, and increment failed count
		logger.Errorf("invalid request body, should be application/cbor")
		c.JSON(415, "invalid request body, should be application/cbor")
		_ = ginmetrics.GetMonitor().GetMetric("tx_submit_fail_count").Inc(nil)
		return
	}
	// Read raw transaction bytes from the request body and store in a byte array
	txRawBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		// Log the error, return an error to the user, and increment failed count
		logger.Errorf("failed to read request body: %s", err)
		c.JSON(500, "failed to read request body")
		_ = ginmetrics.GetMonitor().GetMetric("tx_submit_fail_count").Inc(nil)
		return
	}
	// Close request body after read
	if c.Request.Body != nil {
		if err := c.Request.Body.Close(); err != nil {
			logger.Errorf("failed to close request body: %s", err)
		}
	}
	// Send TX
	errorChan := make(chan error)
	submitConfig := &submit.Config{
		ErrorChan:    errorChan,
		NetworkMagic: cfg.Node.NetworkMagic,
		NodeAddress:  cfg.Node.Address,
		NodePort:     cfg.Node.Port,
		SocketPath:   cfg.Node.SocketPath,
		Timeout:      cfg.Node.Timeout,
	}
	txHash, err := submit.SubmitTx(submitConfig, txRawBytes)
	if err != nil {
		if c.GetHeader("Accept") == "application/cbor" {
			txRejectErr := err.(localtxsubmission.TransactionRejectedError)
			c.Data(400, "application/cbor", txRejectErr.ReasonCbor)
		} else {
			if err.Error() != "" {
				c.JSON(400, err.Error())
			} else {
				c.JSON(400, fmt.Sprintf("%s", err))
			}
		}
		_ = ginmetrics.GetMonitor().GetMetric("tx_submit_fail_count").Inc(nil)
		return
	}
	// Start async error handler
	go func() {
		err, ok := <-errorChan
		if ok {
			logger.Errorf("failure communicating with node: %s", err)
			c.JSON(500, "failure communicating with node")
			_ = ginmetrics.GetMonitor().
				GetMetric("tx_submit_fail_count").
				Inc(nil)
		}
	}()
	// Return transaction ID
	c.JSON(202, txHash)
	// Increment custom metric
	_ = ginmetrics.GetMonitor().GetMetric("tx_submit_count").Inc(nil)
}
