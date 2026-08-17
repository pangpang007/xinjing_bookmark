# Build stage
FROM golang:1.22-alpine AS builder

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ENV HTTP_PROXY=$HTTP_PROXY \
    HTTPS_PROXY=$HTTPS_PROXY \
    NO_PROXY=$NO_PROXY \
    GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /app/main .

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata wget \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo Asia/Shanghai > /etc/timezone

WORKDIR /app

COPY --from=builder /app/main .
COPY assets ./assets

RUN mkdir -p assets/fonts \
    && if [ ! -f assets/fonts/SourceHanSansCN-Regular.otf ]; then \
         wget -q -O assets/fonts/SourceHanSansCN-Regular.otf \
           https://cdn.jsdelivr.net/gh/notofonts/noto-cjk@Sans2.004/Sans/SubsetOTF/SC/NotoSansSC-Regular.otf \
         || wget -q -O assets/fonts/SourceHanSansCN-Regular.otf \
           https://github.com/notofonts/noto-cjk/raw/Sans2.004/Sans/SubsetOTF/SC/NotoSansSC-Regular.otf \
         || echo "font download failed, share-image will be disabled"; \
       fi

ENV TZ=Asia/Shanghai
EXPOSE 8080
CMD ["./main"]
