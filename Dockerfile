# syntax=docker/dockerfile:1

FROM node:24.22.2-bookworm-slim AS web-build
WORKDIR /src
COPY server ./server
COPY web ./web
WORKDIR /src/web
RUN npm ci && npm run build

FROM golang:1.27.3-bookworm AS go-build
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server ./
COPY --from=web-build /src/server/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/spotreba ./cmd/spotreba

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-build /out/spotreba /spotreba
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/spotreba"]
CMD ["serve"]
