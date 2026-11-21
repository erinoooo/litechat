FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS bin
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /litechat .

FROM scratch
COPY --from=bin /litechat /litechat
COPY --from=bin /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENV LITECHAT_DATA=/data
VOLUME /data
EXPOSE 3000
ENTRYPOINT ["/litechat"]
