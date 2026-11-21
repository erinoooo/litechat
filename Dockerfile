# Multi-arch image: the frontend and Go stages run on the build machine and
# cross-compile, so arm64 images don't need emulation.
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS bin
ARG TARGETOS TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /litechat .

FROM scratch
COPY --from=bin /litechat /litechat
COPY --from=bin /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENV LITECHAT_DATA=/data
VOLUME /data
EXPOSE 3000
ENTRYPOINT ["/litechat"]
