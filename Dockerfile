FROM golang:latest AS builder
COPY . /src

WORKDIR /src

RUN CGO_ENABLED=0 go build -ldflags="-extldflags=-static -s -w" -o fdroidswh

FROM alpine:latest

COPY --from=builder /src/fdroidswh /app/fdroidswh

RUN chmod +x /app/fdroidswh

WORKDIR /app

CMD ["/app/fdroidswh"]
