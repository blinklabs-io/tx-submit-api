package api

import (
	"encoding/hex"
	"fmt"
	"github.com/cloudstruct/go-cardano-submit-api/internal/config"
	"github.com/cloudstruct/go-cardano-submit-api/internal/logging"
	ouroboros "github.com/cloudstruct/go-ouroboros-network"
	"github.com/cloudstruct/go-ouroboros-network/block"
	"github.com/cloudstruct/go-ouroboros-network/protocol/localtxsubmission"
	"github.com/fxamacker/cbor/v2"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	"golang.org/x/crypto/blake2b"
	"io/ioutil"
)

func Start(cfg *config.Config) error {
	// Disable gin debug output
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()

	// Configure router
	router := gin.New()
	// Catch panics and return a 500
	router.Use(gin.Recovery())
	// Access logging
	accessLogger := logging.GetAccessLogger()
	router.Use(ginzap.Ginzap(accessLogger, "", true))
	router.Use(ginzap.RecoveryWithZap(accessLogger, true))

	// Create a healthcheck (before metrics so it's not instrumented)
	router.GET("/healthcheck", handleHealthcheck)

	// Metrics
	metricsRouter := gin.New()
	metrics := ginmetrics.GetMonitor()
	// Set metrics router
	metrics.Expose(metricsRouter)
	// Set metrics path
	metrics.SetMetricPath("/")
	// Use metrics middleware without exposing path in main app router
	metrics.UseWithoutExposingEndpoint(router)

	// Custom metrics
	failureMetric := &ginmetrics.Metric{
		// This is a Gauge because input-output-hk's is a gauge
		Type:        ginmetrics.Gauge,
		Name:        "tx_failure_count",
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

	// Start metrics listener
	go func() {
		// TODO: return error if we cannot initialize metrics
		_ = metricsRouter.Run(fmt.Sprintf("%s:%d",
			cfg.Metrics.MetricsAddress,
			cfg.Metrics.MetricsPort))
	}()

	// Configure API routes
	router.POST("/api/submit/tx", handleSubmitTx)

	// Start API listener
	err := router.Run(fmt.Sprintf("%s:%d",
		cfg.Api.ListenAddress,
		cfg.Api.ListenPort))
	return err
}

func handleHealthcheck(c *gin.Context) {
	// TODO: add some actual health checking here
	c.JSON(200, gin.H{"failed": false})
}

func handleSubmitTx(c *gin.Context) {
	cfg := config.GetConfig()
	logger := logging.GetLogger()
	// Read transaction from request body
	rawTx, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		logger.Errorf("failed to read request body: %s", err)
		c.String(500, "failed to request body")
		_ = ginmetrics.GetMonitor().GetMetric("tx_failure_count").Inc(nil)
		return
	}
	if err := c.Request.Body.Close(); err != nil {
		logger.Errorf("failed to close request body: %s", err)
	}
	// Unwrap transaction and calculate ID
	var txUnwrap []cbor.RawMessage
	if err := cbor.Unmarshal(rawTx, &txUnwrap); err != nil {
		logger.Errorf("failed to unwrap transaction CBOR: %s", err)
		c.String(400, fmt.Sprintf("failed to unwrap transaction CBOR: %s", err))
		_ = ginmetrics.GetMonitor().GetMetric("tx_failure_count").Inc(nil)
		return
	}
	txId := blake2b.Sum256(txUnwrap[0])
	txIdHex := hex.EncodeToString(txId[:])
	// Connect to cardano-node and submit TX
	errorChan := make(chan error)
	doneChan := make(chan bool)
	oOpts := &ouroboros.OuroborosOptions{
		NetworkMagic:          uint32(cfg.Node.NetworkMagic),
		ErrorChan:             errorChan,
		UseNodeToNodeProtocol: false,
		LocalTxSubmissionCallbackConfig: &localtxsubmission.CallbackConfig{
			AcceptTxFunc: func() error {
				// Return transaction ID
				c.String(202, txIdHex)
				doneChan <- true
				// Increment custom metric
				_ = ginmetrics.GetMonitor().GetMetric("tx_submit_count").Inc(nil)
				return nil
			},
			RejectTxFunc: func(reason interface{}) error {
				c.String(400, fmt.Sprintf("transaction rejected by node: %#v", reason))
				doneChan <- true
				// Increment custom metric
				_ = ginmetrics.GetMonitor().GetMetric("tx_failure_count").Inc(nil)
				return nil
			},
		},
	}
	oConn, err := ouroboros.New(oOpts)
	defer func() {
		// We have to close the channel to break out of the async error handler goroutine
		close(errorChan)
		// Close Ouroboros connection
		oConn.Close()
	}()
	if err != nil {
		logger.Errorf("failure creating Ouroboros connection: %s", err)
		c.String(500, "failure communicating with node")
		_ = ginmetrics.GetMonitor().GetMetric("tx_failure_count").Inc(nil)
		return
	}
	if cfg.Node.Address != "" && cfg.Node.Port > 0 {
		if err := oConn.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Node.Address, cfg.Node.Port)); err != nil {
			logger.Errorf("failure connecting to node via TCP: %s", err)
			c.String(500, "failure communicating with node")
			_ = ginmetrics.GetMonitor().GetMetric("tx_failure_count").Inc(nil)
			return
		}
	} else {
		if err := oConn.Dial("unix", cfg.Node.SocketPath); err != nil {
			logger.Errorf("failure connecting to node via UNIX socket: %s", err)
			c.String(500, "failure communicating with node")
			_ = ginmetrics.GetMonitor().GetMetric("tx_failure_count").Inc(nil)
			return
		}
	}
	// Start async error handler
	go func() {
		err, ok := <-errorChan
		if ok {
			logger.Errorf("failure communicating with node: %s", err)
			c.String(500, "failure communicating with node")
			_ = ginmetrics.GetMonitor().GetMetric("tx_failure_count").Inc(nil)
			doneChan <- true
		}
	}()
	// TODO: figure out better way to determine era
	if err = oConn.LocalTxSubmission.SubmitTx(block.TX_TYPE_ALONZO, rawTx); err != nil {
		logger.Errorf("failure submitting transaction: %s", err)
		c.String(500, "failure communicating with node")
		return
	}
	// Wait for async process to finish
	<-doneChan
}
