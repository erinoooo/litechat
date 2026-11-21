// Package discover finds local AI servers (Ollama, LM Studio, llama.cpp,
// vLLM, KoboldCpp, …) by probing their default ports on this machine and,
// on request, on the local network. Every server found speaks the OpenAI
// API, so it can be added as an "openai" provider as-is.
package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Kind    string   `json:"kind"` // "Ollama", "LM Studio", … or "OpenAI-compatible server"
	Host    string   `json:"host"` // as reachable from liteChat
	Port    int      `json:"port"`
	BaseURL string   `json:"base_url"`
	Local   bool     `json:"local"` // on this machine (or its Docker host)
	Version string   `json:"version,omitempty"`
	Models  []string `json:"models"`
}

// signature recognises one kind of server. identify returns ok when the
// server at base (scheme://host:port) is that kind.
type signature struct {
	kind     string
	ports    []int
	identify func(ctx context.Context, base string) (version string, ok bool)
}

var signatures = []signature{
	{"Ollama", []int{11434}, func(ctx context.Context, base string) (string, bool) {
		var v struct{ Version string }
		ok := getJSON(ctx, base+"/api/version", &v) && v.Version != ""
		return v.Version, ok
	}},
	{"LM Studio", []int{1234}, func(ctx context.Context, base string) (string, bool) {
		// LM Studio's own REST API lists models with a "type" (llm, vlm, embeddings)
		var v struct {
			Data []struct{ Type string } `json:"data"`
		}
		ok := getJSON(ctx, base+"/api/v0/models", &v) && v.Data != nil
		return "", ok
	}},
	{"KoboldCpp", []int{5001}, func(ctx context.Context, base string) (string, bool) {
		var v struct{ Result, Version string }
		ok := getJSON(ctx, base+"/api/extra/version", &v) && v.Result == "KoboldCpp"
		return v.Version, ok
	}},
	{"llama.cpp", []int{8080}, func(ctx context.Context, base string) (string, bool) {
		var v map[string]json.RawMessage
		if !getJSON(ctx, base+"/props", &v) {
			return "", false
		}
		_, ok := v["default_generation_settings"]
		var build string
		_ = json.Unmarshal(v["build_info"], &build)
		return build, ok
	}},
	{"SGLang", []int{30000}, func(ctx context.Context, base string) (string, bool) {
		var v struct {
			ModelPath string `json:"model_path"`
		}
		return "", getJSON(ctx, base+"/get_model_info", &v) && v.ModelPath != ""
	}},
	{"Text Generation WebUI", []int{5000}, func(ctx context.Context, base string) (string, bool) {
		var v struct {
			ModelName string `json:"model_name"`
		}
		return "", getJSON(ctx, base+"/v1/internal/model/info", &v) && v.ModelName != ""
	}},
	{"LiteLLM", []int{4000}, func(ctx context.Context, base string) (string, bool) {
		var alive any
		var ml modelList
		return "", getJSON(ctx, base+"/health/liveliness", &alive) && getJSON(ctx, base+"/v1/models", &ml) && ml.Data != nil
	}},
}

// owned_by values some servers put in /v1/models, for servers without a
// distinctive endpoint of their own.
var ownedBy = map[string]string{
	"vllm":      "vLLM",
	"tabbyapi":  "TabbyAPI",
	"llamacpp":  "llama.cpp",
	"aphrodite": "Aphrodite",
	"sglang":    "SGLang",
	"localai":   "LocalAI",
	"mlx":       "MLX",
	"gpt4all":   "GPT4All",
}

// A port used by exactly one well-known app is a good hint when nothing else identifies it.
var portHint = map[int]string{1337: "Jan", 4891: "GPT4All", 2242: "Aphrodite", 8000: "vLLM"}

// Ports probes check, roughly in order of popularity.
var Ports = []int{11434, 1234, 8080, 8000, 5000, 5001, 1337, 4891, 30000, 2242, 4000, 8001}

// noProxy: local probes must never go through an HTTP proxy from the environment.
var client = &http.Client{
	Timeout:   1500 * time.Millisecond,
	Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true, MaxIdleConns: 0},
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return client.Do(req)
}

func getJSON(ctx context.Context, url string, v any) bool {
	resp, err := get(ctx, url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(v) == nil
}

type modelList struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// Probe checks one host:port and reports the server there, if any.
func Probe(ctx context.Context, host string, port int) (*Server, bool) {
	hp := net.JoinHostPort(host, strconv.Itoa(port))
	base := "http://" + hp

	// signatures that claim this port go first; then everything else, so a
	// server on a non-default port is still recognised
	sigs := make([]signature, 0, len(signatures))
	for _, s := range signatures {
		if containsPort(s.ports, port) {
			sigs = append(sigs, s)
		}
	}
	for _, s := range signatures {
		if !containsPort(s.ports, port) {
			sigs = append(sigs, s)
		}
	}

	s := &Server{Host: host, Port: port, BaseURL: base + "/v1", Models: []string{}}
	for _, sig := range sigs {
		if v, ok := sig.identify(ctx, base); ok {
			s.Kind, s.Version = sig.kind, v
			break
		}
	}

	var ml modelList
	hasModels := getJSON(ctx, base+"/v1/models", &ml) && ml.Data != nil
	if s.Kind == "" && !hasModels {
		return nil, false // something is listening, but it isn't an OpenAI-style AI server
	}
	for _, m := range ml.Data {
		if s.Kind == "" {
			if k, ok := ownedBy[strings.ToLower(m.OwnedBy)]; ok {
				s.Kind = k
			}
		}
		if len(s.Models) < 50 {
			s.Models = append(s.Models, m.ID)
		}
	}
	if s.Kind == "" {
		if k, ok := portHint[port]; ok {
			s.Kind = k
		} else {
			s.Kind = "OpenAI-compatible server"
		}
	}
	return s, true
}

func containsPort(ps []int, p int) bool {
	for _, x := range ps {
		if x == p {
			return true
		}
	}
	return false
}

// open reports whether something accepts TCP connections at host:port.
func open(ctx context.Context, host string, port int, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	c.Close()
	return true
}

type Options struct {
	Network  bool // also scan the local network (/24 around each private IPv4 address)
	SkipPort int  // liteChat's own port on this machine
}

type Result struct {
	Servers []Server `json:"servers"`
	Scanned int      `json:"scanned"` // host:port pairs tried
	TookMS  int64    `json:"took_ms"`
}

// Scan probes this machine (and its Docker host), plus the LAN if asked.
func Scan(ctx context.Context, opt Options) Result {
	start := time.Now()
	type target struct {
		host    string
		local   bool
		timeout time.Duration
	}
	var targets []target
	for _, h := range localHosts() {
		targets = append(targets, target{h, true, 300 * time.Millisecond})
	}
	if opt.Network {
		for _, h := range lanHosts(4) {
			targets = append(targets, target{h, false, 300 * time.Millisecond})
		}
	}

	var (
		mu      sync.Mutex
		found   []Server
		seen    = map[string]bool{}
		wg      sync.WaitGroup
		sem     = make(chan struct{}, 512)
		scanned int
	)
	for _, t := range targets {
		for _, port := range Ports {
			if t.local && port == opt.SkipPort {
				continue
			}
			scanned++
			wg.Add(1)
			sem <- struct{}{}
			go func(t target, port int) {
				defer wg.Done()
				defer func() { <-sem }()
				if !open(ctx, t.host, port, t.timeout) {
					return
				}
				s, ok := Probe(ctx, t.host, port)
				if !ok {
					return
				}
				s.Local = t.local
				mu.Lock()
				defer mu.Unlock()
				// the same server can answer on 127.0.0.1 and the Docker host; keep one
				key := fmt.Sprintf("%s|%d|%v", s.Kind, s.Port, s.Local)
				if t.local && seen[key] {
					return
				}
				seen[key] = true
				found = append(found, *s)
			}(t, port)
		}
	}
	wg.Wait()

	sort.Slice(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.Local != b.Local {
			return a.Local
		}
		if a.Host != b.Host {
			return a.Host < b.Host
		}
		return a.Port < b.Port
	})
	if found == nil {
		found = []Server{}
	}
	return Result{Servers: found, Scanned: scanned, TookMS: time.Since(start).Milliseconds()}
}

// localHosts: loopback, plus the Docker host when liteChat runs in a container.
func localHosts() []string {
	hosts := []string{"127.0.0.1"}
	if inContainer() {
		if addrs, err := net.LookupHost("host.docker.internal"); err == nil && len(addrs) > 0 {
			hosts = append(hosts, addrs[0])
		} else if gw := defaultGateway(); gw != "" {
			hosts = append(hosts, gw)
		}
	}
	return hosts
}

func inContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// defaultGateway reads the IPv4 default route (Linux only).
func defaultGateway() string {
	b, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) < 3 || f[1] != "00000000" {
			continue
		}
		v, err := strconv.ParseUint(f[2], 16, 32)
		if err != nil {
			continue
		}
		return net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24)).String()
	}
	return ""
}

// virtualBridge reports interfaces whose neighbours are local VMs or
// containers rather than other machines on the network.
func virtualBridge(name string) bool {
	for _, p := range []string{"docker", "br-", "veth", "virbr", "lxcbr", "lxdbr", "cni", "flannel", "podman", "vmnet", "vboxnet", "tap", "tun", "zt", "wg"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// lanHosts lists every address in the /24 around each private IPv4 address
// of this machine (excluding the machine itself), up to maxNets networks.
func lanHosts(maxNets int) []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	self := map[string]bool{}
	var nets []net.IP
	seenNet := map[string]bool{}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		if virtualBridge(ifc.Name) {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipn.IP.To4()
			if ip == nil || !ip.IsPrivate() {
				continue
			}
			self[ip.String()] = true
			key := ip.Mask(net.CIDRMask(24, 32)).String()
			if !seenNet[key] && len(nets) < maxNets {
				seenNet[key] = true
				nets = append(nets, ip.Mask(net.CIDRMask(24, 32)))
			}
		}
	}
	var hosts []string
	for _, n := range nets {
		for i := 1; i < 255; i++ {
			h := net.IPv4(n[0], n[1], n[2], byte(i)).String()
			if !self[h] {
				hosts = append(hosts, h)
			}
		}
	}
	return hosts
}
