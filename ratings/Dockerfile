FROM golang:1.22 AS builder

WORKDIR /app

COPY . .

RUN go mod download
RUN go build -o /app/ratings

FROM alpine:latest

# Instalar a biblioteca libc6-compat, necessária para compatibilidade com binários Go
RUN apk add --no-cache libc6-compat

COPY --from=builder /app/ratings /ratings

EXPOSE 8085

CMD ["/ratings"]
