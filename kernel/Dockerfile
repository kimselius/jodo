FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o kernel ./cmd/kernel/

FROM alpine:3.19
RUN apk add --no-cache openssh-client git
COPY --from=builder /build/kernel /app/kernel
COPY configs/ /app/configs/
COPY seed/ /app/seed/
WORKDIR /app
CMD ["./kernel"]
