FROM ghcr.io/blinklabs-io/go:1.23.4-1 AS build

WORKDIR /code
COPY . .
RUN make build

FROM cgr.dev/chainguard/glibc-dynamic AS tx-submit-api
COPY --from=build /code/tx-submit-api /bin/
USER root
ENTRYPOINT ["tx-submit-api"]
