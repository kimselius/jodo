# Stage 1: Build Vue SPA
FROM node:22-alpine AS frontend
WORKDIR /build/web
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY web/package.json web/pnpm-lock.yaml* ./
RUN pnpm install --no-frozen-lockfile
COPY web/ .
RUN pnpm build

# Stage 2: Build Go binary
FROM golang:1.22-alpine AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o kernel ./cmd/kernel/

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache openssh-client git
COPY --from=backend /build/kernel /app/kernel
COPY --from=frontend /build/web/dist /app/web
COPY configs/ /app/configs/
COPY seed/ /app/seed/
WORKDIR /app
CMD ["./kernel"]
