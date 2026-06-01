FROM golang:1.26.1-alpine AS build

WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGET=server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${TARGET}

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S app \
	&& adduser -S app -G app

WORKDIR /app

COPY --from=build /out/app /app/app
COPY configs /app/configs

RUN mkdir -p /app/uploads/images \
	&& chown -R app:app /app/uploads

USER app

EXPOSE 8080

ENTRYPOINT ["/app/app"]
