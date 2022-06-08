FROM golang:1.18 AS build

WORKDIR /code
COPY . .
RUN make build

FROM ubuntu:focal AS final
COPY --from=build /code/cardano-submit-api /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/cardano-submit-api"]
