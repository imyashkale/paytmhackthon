# Build any MCP server: docker build --build-arg SERVER=crm -t mcp-crm .
FROM golang:1.27-alpine AS build
ARG SERVER
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY servers ./servers
RUN test -n "$SERVER" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./servers/${SERVER}/cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/server"]
