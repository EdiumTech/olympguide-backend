FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o main ./cmd/main.go

FROM alpine:3.22
RUN sed -i 's@https://dl-cdn.alpinelinux.org/alpine@https://mirror.yandex.ru/mirrors/alpine@g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates netcat-openbsd
WORKDIR /app
COPY --from=builder /app/main .
USER 65532:65532
EXPOSE ${STORAGE_SERVICE_PORT}
CMD ["./main"]
