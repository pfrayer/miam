FROM golang:1.22-alpine@sha256:1699c10032ca2582ec89a24a1312d986a3f094aed3d5c1147b19880afe40e052 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/miam ./cmd/miam \
 && mkdir -p /out/data

FROM scratch
LABEL maintainer="Pierre Frayer <pfrayer@gmail.com>"
# CA bundle: the sync calls Google over HTTPS.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/miam /miam
COPY --from=build --chown=65534:65534 /out/data /data
# The list configuration is not baked in: mount it at /config/config.json
# (see config/config.example.json).
USER 65534:65534
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/miam", "-healthcheck"]
ENTRYPOINT ["/miam", "-config", "/config/config.json", "-data", "/data/snapshot.json"]
