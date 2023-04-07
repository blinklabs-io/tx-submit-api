FROM cgr.dev/chainguard/go:1.19 AS build

WORKDIR /code
COPY . .
RUN make build

FROM cgr.dev/chainguard/glibc-dynamic AS tx-submit-api
COPY --from=build /code/tx-submit-api /bin/
ENTRYPOINT ["tx-submit-api"]
