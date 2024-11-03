FROM golang:1.22 AS builder

WORKDIR /app

COPY . .

RUN go mod download
RUN go build -o /app/details

FROM alpine:latest

# Instalar a biblioteca libc6-compat, necessária para compatibilidade com binários Go
RUN apk add --no-cache libc6-compat

COPY --from=builder /app/details /details

EXPOSE 9084

CMD ["/details"]
