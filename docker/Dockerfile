FROM golang:alpine3.22 AS builder

RUN apk update && apk add --no-cache ca-certificates

COPY . /app

WORKDIR /app

RUN go build -o back ./cmd/main.go

FROM alpine:3.22

# Устанавливаем Docker CLI и ca-certificates
RUN apk add --no-cache docker-cli ca-certificates

# Копируем бинарник и миграции из первого этапа сборки
COPY --from=builder /app/back /back
COPY --from=builder /app/migrate /migrate

# Копируем сертификаты
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Монтирование сокета Docker будет происходить при запуске контейнера
# В entrypoint указываем ваш бинарник

ENTRYPOINT ["/back"]
