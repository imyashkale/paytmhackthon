# Build from the repo root: docker build -f monitor/Dockerfile -t mcp-monitor .
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY monitor ./monitor
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/monitor ./monitor/cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/monitor /monitor
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/monitor"]
