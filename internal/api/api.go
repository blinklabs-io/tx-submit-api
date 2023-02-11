package api

import (
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/cloudstruct/go-cardano-ledger"
	"github.com/cloudstruct/go-cardano-submit-api/internal/config"
	"github.com/cloudstruct/go-cardano-submit-api/internal/logging"
	ouroboros "github.com/cloudstruct/go-ouroboros-network"
	"github.com/cloudstruct/go-ouroboros-network/protocol/localtxsubmission"

	"github.com/fxamacker/cbor/v2"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
	swaggerFiles "github.com/swaggo/files"     // swagger embed files
	ginSwagger "github.com/swaggo/gin-swagger" // gin-swagger middleware
	"golang.org/x/crypto/blake2b"

	_ "github.com/cloudstruct/go-cardano-submit-api/docs" // docs is generated by Swag CLI
)

// @title        go-cardano-submit-api
// @version      3.1.0
// @description  Cardano Submit API
// @host         localhost
// @Schemes      http
// @BasePath     /

// @contact.name   CloudStruct
// @contact.url    https://cloudstruct.net
// @contact.email  support@cloudstruct.net

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html
func Start(cfg *config.Config) error {
	// Disable gin debug and color output
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()

	// Configure API router
	router := gin.New()
	// Catch panics and return a 500
	router.Use(gin.Recovery())
	// Standard logging
	logger := logging.GetLogger()
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

	// Create a healthcheck (before metrics so it's not instrumented)
	router.GET("/healthcheck", handleHealthcheck)
	// Create a swagger endpoint (not instrumented)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Metrics
	metricsRouter := gin.New()
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

// handleSubmitTx godoc
// @Summary      Submit Tx
// @Description  Submit an already serialized transaction to the network.
// @Produce      json
// @Param        Content-Type  header    string  true  "Content type"  Enums(application/cbor)
// @Success      202           {object}  string  "Ok"
// @Failure      400           {object}  string  "Bad Request"
// @Failure      415           {object}  string  "Unsupported Media Type"
// @Failure      500           {object}  string  "Server Error"
// @Router       /api/submit/tx [post]
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
	if err := c.Request.Body.Close(); err != nil {
		logger.Errorf("failed to close request body: %s", err)
	}
	// Unwrap raw transaction bytes into a CBOR array
	var txUnwrap []cbor.RawMessage
	if err := cbor.Unmarshal(txRawBytes, &txUnwrap); err != nil {
		logger.Errorf("failed to unwrap transaction CBOR: %s", err)
		c.JSON(400, fmt.Sprintf("failed to unwrap transaction CBOR: %s", err))
		_ = ginmetrics.GetMonitor().GetMetric("tx_submit_fail_count").Inc(nil)
		return
	}
	// index 0 is the transaction body
	// Store index 0 (transaction body) as byte array
	txBody := txUnwrap[0]

	// Convert the body into a blake2b256 hash string
	txIdHash := blake2b.Sum256(txBody)
	// Encode hash string as byte array to hex string
	txIdHex := hex.EncodeToString(txIdHash[:])
	// Connect to cardano-node and submit TX
	errorChan := make(chan error)
	oConn, err := ouroboros.New(
		ouroboros.WithNetworkMagic(uint32(cfg.Node.NetworkMagic)),
		ouroboros.WithErrorChan(errorChan),
		ouroboros.WithNodeToNode(false),
		ouroboros.WithLocalTxSubmissionConfig(
			localtxsubmission.NewConfig(
				localtxsubmission.WithTimeout(5*time.Second),
			),
		),
	)
	if err != nil {
		logger.Errorf("failure creating Ouroboros connection: %s", err)
		c.JSON(500, "failure communicating with node")
		_ = ginmetrics.GetMonitor().GetMetric("tx_submit_fail_count").Inc(nil)
		return
	}
	if cfg.Node.Address != "" && cfg.Node.Port > 0 {
		if err := oConn.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Node.Address, cfg.Node.Port)); err != nil {
			logger.Errorf("failure connecting to node via TCP: %s", err)
			c.JSON(500, "failure communicating with node")
			_ = ginmetrics.GetMonitor().GetMetric("tx_submit_fail_count").Inc(nil)
			return
		}
	} else {
		if err := oConn.Dial("unix", cfg.Node.SocketPath); err != nil {
			logger.Errorf("failure connecting to node via UNIX socket: %s", err)
			c.JSON(500, "failure communicating with node")
			_ = ginmetrics.GetMonitor().GetMetric("tx_submit_fail_count").Inc(nil)
			return
		}
	}
	// Start async error handler
	go func() {
		err, ok := <-errorChan
		if ok {
			logger.Errorf("failure communicating with node: %s", err)
			c.JSON(500, "failure communicating with node")
			_ = ginmetrics.GetMonitor().GetMetric("tx_submit_fail_count").Inc(nil)
		}
	}()
	defer func() {
		// Close Ouroboros connection
		oConn.Close()
	}()
	// Start local-tx-submission protocol
	oConn.LocalTxSubmission().Client.Start()
	// Determine transaction type (era)
	txType, err := determineTransactionType(txRawBytes)
	if err != nil {
		c.JSON(400, "could not parse transaction to determine type")
		return
	}
	// Submit the transaction
	if err := oConn.LocalTxSubmission().Client.SubmitTx(txType, txRawBytes); err != nil {
		if c.GetHeader("Accept") == "application/cbor" {
			txRejectErr := err.(localtxsubmission.TransactionRejectedError)
			c.Data(400, "application/cbor", txRejectErr.ReasonCbor)
		} else {
			c.JSON(400, err.Error())
		}
		// Increment custom metric
		_ = ginmetrics.GetMonitor().GetMetric("tx_submit_fail_count").Inc(nil)
		return
	}
	// Return transaction ID
	c.JSON(202, txIdHex)
	// Increment custom metric
	_ = ginmetrics.GetMonitor().GetMetric("tx_submit_count").Inc(nil)
}

func determineTransactionType(data []byte) (uint16, error) {
	// TODO: uncomment this once the following issue is resolved:
	// https://github.com/cloudstruct/go-cardano-ledger/issues/9
	/*
		if _, err := ledger.NewByronTransactionFromCbor(data); err == nil {
			return ledger.TX_TYPE_BYRON, nil
		}
	*/
	if _, err := ledger.NewShelleyTransactionFromCbor(data); err == nil {
		return ledger.TX_TYPE_SHELLEY, nil
	}
	if _, err := ledger.NewAllegraTransactionFromCbor(data); err == nil {
		return ledger.TX_TYPE_ALLEGRA, nil
	}
	if _, err := ledger.NewMaryTransactionFromCbor(data); err == nil {
		return ledger.TX_TYPE_MARY, nil
	}
	if _, err := ledger.NewAlonzoTransactionFromCbor(data); err == nil {
		return ledger.TX_TYPE_ALONZO, nil
	}
	if _, err := ledger.NewBabbageTransactionFromCbor(data); err == nil {
		return ledger.TX_TYPE_BABBAGE, nil
	}
	return 0, fmt.Errorf("unknown transaction type")
}
