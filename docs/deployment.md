# Deploying

liteChat is one binary that serves the app and the API on one port. It has no TLS of its own: on a trusted network you can use it directly, and anywhere else put it behind a reverse proxy that handles HTTPS.

## systemd

```ini
# /etc/systemd/system/litechat.service
[Unit]
Description=liteChat
After=network-online.target
Wants=network-online.target

[Service]
User=litechat
ExecStart=/usr/local/bin/litechat --host 127.0.0.1 --port 3000 --data /var/lib/litechat
Restart=on-failure
# Optional: set the password from a secret file holding LITECHAT_PASSWORD=...
# EnvironmentFile=/etc/litechat.env
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
StateDirectory=litechat
ReadWritePaths=/var/lib/litechat

[Install]
WantedBy=multi-user.target
```

```sh
sudo useradd --system --home /var/lib/litechat litechat
sudo cp litechat /usr/local/bin/
sudo systemctl enable --now litechat
```

Remove `ProtectHome=true` if a local MCP server needs to read home directories.

## Reverse proxy

liteChat streams replies with server-sent events, so the proxy must not buffer responses. liteChat sends `X-Accel-Buffering: no`, which nginx honours; Caddy doesn't buffer by default.

Set `X-Forwarded-Proto: https` so the login cookie is marked secure.

### Caddy

```
chat.example.com {
    reverse_proxy 127.0.0.1:3000
}
```

### nginx

```nginx
server {
    listen 443 ssl;
    server_name chat.example.com;
    # ssl_certificate …; ssl_certificate_key …;

    client_max_body_size 64m;   # image and document uploads

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
        proxy_read_timeout 1h;   # long replies
    }
}
```

## Docker

```sh
docker run -d --name litechat -p 3000:3000 -v litechat:/data \
  --add-host=host.docker.internal:host-gateway \
  ghcr.io/erinoooo/litechat:latest
```

Inside the container, the host is `host.docker.internal`, so Ollama on the host is `http://host.docker.internal:11434/v1`. Detected servers finds it automatically. Make sure Ollama listens on more than localhost (`OLLAMA_HOST=0.0.0.0`) or the container can't reach it.

The image is built `FROM scratch`: it contains only liteChat and CA certificates. Images are published for `linux/amd64` and `linux/arm64`.

## Security checklist

- Use a long password, or set it from a secret with `LITECHAT_PASSWORD` (not `--password`, which other users on the machine can see in the process list).
- Serve it over HTTPS if it's reachable beyond your own network. Login attempts are rate-limited per IP (10 failures per 10 minutes).
- Provider and MCP URLs are requested by the server, so anyone who can log in can make liteChat send requests to addresses it can reach.
- Leave `--allow-local-mcp` off unless you need local MCP servers.
- The database holds API keys: restrict access to the `data` folder and its backups.
