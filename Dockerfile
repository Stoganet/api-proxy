FROM golang:1.26@sha256:68cb6d68bed024785b69195b89af7ac7a444f27791435f98647edff595aa0479 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api-proxy ./cmd/api-proxy
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian13:nonroot@sha256:963fa6c544fe5ce420f1f54fb88b6fb01479f054c8056d0f74cc2c6000df5240
COPY --from=build /out/api-proxy /api-proxy
COPY --from=build --chown=65532:65532 /out/data /data
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api-proxy"]
