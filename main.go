// liteChat: a lightweight, self-hosted chat UI for LLMs.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"litechat/internal/db"
	"litechat/internal/server"
)

//go:embed all:web/dist
var webDist embed.FS

// version is set at build time: -ldflags "-X main.version=v0.1.0"
var version = "dev"

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// defaultDataDir keeps using ./data when it already holds a database (how
// v0.1.0 worked), and otherwise uses a per-user folder, so the command works
// the same from any directory: ~/.local/share/litechat on Linux,
// ~/Library/Application Support/litechat on macOS, %LocalAppData%\litechat
// on Windows.
func defaultDataDir() string {
	if _, err := os.Stat(filepath.Join("data", "litechat.db")); err == nil {
		return "data"
	}
	return userDataDir()
}

func userDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "data"
	}
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("LocalAppData"); d != "" {
			return filepath.Join(d, "litechat")
		}
		return filepath.Join(home, "AppData", "Local", "litechat")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "litechat")
	default:
		if d := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(d) {
			return filepath.Join(d, "litechat")
		}
		return filepath.Join(home, ".local", "share", "litechat")
	}
}

const usage = `liteChat %s: a lightweight chat UI for local and cloud LLMs.

Usage: litechat [options]

Options:
  --port N            port to listen on (default 3000; 0 picks a free one)
  --host ADDR         interface to listen on, e.g. 127.0.0.1 for this
                      machine only (default: all interfaces)
  --data DIR          folder for the database
                      (default %s)
  --password PW       set or reset the password at startup. Other users
                      on this machine can see command-line arguments, so
                      on shared machines use LITECHAT_PASSWORD instead
  --allow-local-mcp   allow MCP tool servers that run local commands
  --version           print the version and exit

Every option can also be set with an environment variable, which is handy
for Docker: LITECHAT_PORT, LITECHAT_HOST, LITECHAT_DATA, LITECHAT_PASSWORD,
LITECHAT_MCP_STDIO=1. Options on the command line win.
`

// config is the startup configuration: command-line options, falling back
// to LITECHAT_* environment variables, falling back to defaults.
type config struct {
	addr, dataDir, password string
	allowLocalMCP, version  bool
}

func parseConfig(args []string) (config, error) {
	var c config
	fs := flag.NewFlagSet("litechat", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprintf(os.Stdout, usage, version, defaultDataDir()) }
	port := fs.String("port", env("LITECHAT_PORT", "3000"), "")
	host := fs.String("host", env("LITECHAT_HOST", ""), "")
	addr := fs.String("addr", "", "") // host:port in one; kept for v0.1 scripts
	fs.StringVar(&c.dataDir, "data", env("LITECHAT_DATA", ""), "")
	fs.StringVar(&c.password, "password", os.Getenv("LITECHAT_PASSWORD"), "")
	fs.BoolVar(&c.allowLocalMCP, "allow-local-mcp", os.Getenv("LITECHAT_MCP_STDIO") == "1", "")
	fs.BoolVar(&c.version, "version", false, "")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if fs.NArg() > 0 {
		return c, fmt.Errorf("unexpected argument %q (see litechat --help)", fs.Arg(0))
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if p, err := strconv.Atoi(*port); err != nil || p < 0 || p > 65535 {
		return c, fmt.Errorf("invalid port %q", *port)
	}
	switch {
	case *addr != "":
		c.addr = *addr
	case !set["port"] && !set["host"] && os.Getenv("LITECHAT_ADDR") != "":
		c.addr = os.Getenv("LITECHAT_ADDR") // v0.1 variable
	default:
		c.addr = net.JoinHostPort(*host, *port)
	}
	if c.dataDir == "" {
		c.dataDir = defaultDataDir()
	}
	return c, nil
}

// browseURL is the address to open in a browser for a listener.
func browseURL(host string, ln net.Listener) string {
	port := ln.Addr().(*net.TCPAddr).Port
	if h := net.ParseIP(host); host == "" || (h != nil && h.IsUnspecified()) {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}

func main() {
	log.SetFlags(log.LstdFlags)
	c, err := parseConfig(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "litechat:", err)
		os.Exit(2)
	}
	if c.version {
		fmt.Println("litechat", version)
		return
	}

	if err := os.MkdirAll(c.dataDir, 0o700); err != nil {
		log.Fatal(err)
	}
	d, err := db.Open(filepath.Join(c.dataDir, "litechat.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	if c.password != "" && !server.PasswordMatches(d, c.password) {
		if err := server.SetPassword(d, c.password); err != nil {
			log.Fatal(err)
		}
		log.Print("password set")
	}

	// listen before anything else so a busy port fails fast, and so port 0
	// resolves to the real port
	ln, err := net.Listen("tcp", c.addr)
	if err != nil {
		log.Fatal(err)
	}
	host, _, _ := net.SplitHostPort(c.addr)

	static, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatal(err)
	}

	app := server.New(d, static, server.Options{
		AllowStdioMCP: c.allowLocalMCP,
		ListenPort:    ln.Addr().(*net.TCPAddr).Port,
	})
	defer app.Close()
	srv := &http.Server{Handler: app, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		// the Python package waits for this line; keep "listening on <url>"
		log.Printf("liteChat %s listening on %s, data in %s", version, browseURL(host, ln), c.dataDir)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
