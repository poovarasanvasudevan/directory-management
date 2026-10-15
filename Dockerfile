FROM oven/bun:1.3 AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile
COPY frontend ./
RUN bun run build

FROM golang:1.26-alpine AS api
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -o /directory-desk ./pkg/cmd

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=api /directory-desk /usr/local/bin/directory-desk
COPY config.yaml ./config.yaml
USER app
EXPOSE 8080
ENTRYPOINT ["directory-desk"]
