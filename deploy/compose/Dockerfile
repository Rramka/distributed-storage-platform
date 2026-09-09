FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=gateway
RUN CGO_ENABLED=0 go build -o /out/service ./cmd/${SERVICE}

FROM alpine:3.21
RUN apk add --no-cache wget \
    && adduser -D -H -u 10001 dsp
USER dsp
COPY --from=build /out/service /service
EXPOSE 8080
ENTRYPOINT ["/service"]
