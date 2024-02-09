FROM golang:alpine AS builder

WORKDIR /app
COPY . /app

RUN go mod download
RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /service

FROM alpine as final

COPY --from=builder /service /service

ENTRYPOINT [ "/service" ]