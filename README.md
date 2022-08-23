# go-cardano-submit-api
Cardano Transaction Submission API

A simple HTTP API which accepts a CBOR encoded Cardano transaction as a
payload body and submits it to a Cardano full node using the Ouroboros
LocalTxSubmission Node-to-Client (NtC) protocol.

## Usage
The recommended method of using this application is via the published
container images.

```
docker run -p 8090 ghcr.io/cloudstruct/cardano-submit-api:0.4.0
```

### Configuration
Configuration can be done using either a `config.yaml` file or setting
environment variables. Our recommendation is environment variables.

#### Environment variables
Configuration via environment variables can be broken into two sets of
variables. The first set controls the behavior of the application, while the
second set controls the connection to the Cardano node instance.

Application configuration:
- `API_LISTEN_ADDRESS` - Address to bind for API calls, all addresses if empty
    (default: empty)
- `API_LISTEN_PORT` - Port to bind for API calls (default: 8090)
- `DEBUG_ADDRESS` - Address to bind for pprof debugging (default: localhost)
- `DEBUG_PORT` - Port to bind for pprof debugging, disabled if 0 (default: 0)
- `LOGGING_LEVEL` - Logging level for log output (default: info)
- `METRICS_LISTEN_ADDRESS` - Address to bind for Prometheus format metrics, all
    addresses if empty (default: empty)
- `METRICS_LISTEN_PORT` - Port to bind for metrics (default: 8081)

Connection to the Cardano node can be performed using specific named network
shortcuts for known network magic configurations. Supported named networks are:

- mainnet
- preprod
- preview
- testnet

You can set the network to an empty value and provide your own network magic to
connect to unlisted networks.

TCP connection to a Cardano node without using an intermediary like SOCAT is
possible using the node address and port. It is up to you to expose the node's
NtC communication socket over TCP. TCP connections are preferred over socket
within the application.

Cardano node configuration:
- `CARDANO_NETWORK` - Use a named Cardano network (default: mainnet)
- `CARDANO_NODE_NETWORK_MAGIC` - Cardano network magic (default: automatically
    determined from named network)
- `CARDANO_NODE_SOCKET_PATH` - Socket path to Cardano node NtC via UNIX socket
    (default: /node-ipc/node.socket)
- `CARDANO_NODE_SOCKET_TCP_HOST` - Address to Cardano node NtC via TCP
   (default: unset)
- `CARDANO_NODE_SOCKET_TCP_PORT` - Port to Cardano node NtC via TCP (default:
    unset)
