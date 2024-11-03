FROM golang:1.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o productpage

FROM alpine:latest

RUN apk add --no-cache libc6-compat

WORKDIR /app
COPY --from=builder /app/productpage /app/productpage
COPY --from=builder /app/static /app/static
COPY --from=builder /app/templates /app/templates

EXPOSE 8083

CMD ["/app/productpage"]
