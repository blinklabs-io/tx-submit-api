FROM cgr.dev/chainguard/go:1.19 AS build

WORKDIR /code
COPY . .
RUN make build

FROM cgr.dev/chainguard/glibc-dynamic AS cardano-submit-api
COPY --from=build /code/cardano-submit-api /bin/
ENTRYPOINT ["/bin/cardano-submit-api"]
