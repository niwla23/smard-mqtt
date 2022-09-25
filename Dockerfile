FROM golang:1.19-alpine as builder

WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /smard-mqtt

FROM alpine:3.16
COPY --from=builder /smard-mqtt /smard-mqtt
ENTRYPOINT ["/smard-mqtt"]