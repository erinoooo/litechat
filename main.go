// liteChat: a lightweight, self-hosted chat UI for LLMs.
package main

import (
	"context"
	"embed"
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

func main() {
	addr := flag.String("addr", env("LITECHAT_ADDR", ":3000"), "listen address")
	dataDir := flag.String("data", env("LITECHAT_DATA", defaultDataDir()), "data directory")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("litechat", version)
		return
	}

	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		log.Fatal(err)
	}
	d, err := db.Open(filepath.Join(*dataDir, "litechat.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	// LITECHAT_PASSWORD (optional) sets or resets the password at startup.
	if pw := os.Getenv("LITECHAT_PASSWORD"); pw != "" && !server.PasswordMatches(d, pw) {
		if err := server.SetPassword(d, pw); err != nil {
			log.Fatal(err)
		}
		log.Print("password set from LITECHAT_PASSWORD")
	}

	static, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatal(err)
	}

	listenPort := 0
	if _, p, err := net.SplitHostPort(*addr); err == nil {
		listenPort, _ = strconv.Atoi(p)
	}
	app := server.New(d, static, server.Options{
		AllowStdioMCP: os.Getenv("LITECHAT_MCP_STDIO") == "1",
		ListenPort:    listenPort,
	})
	defer app.Close()
	srv := &http.Server{
		Addr:              *addr,
		Handler:           app,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("liteChat %s listening on %s, data in %s", version, *addr, *dataDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
