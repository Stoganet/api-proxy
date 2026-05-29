FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api-proxy ./cmd/api-proxy

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/api-proxy /api-proxy
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api-proxy"]
