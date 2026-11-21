package main

import "testing"

func TestParseConfig(t *testing.T) {
	cases := []struct {
		args []string
		env  map[string]string
		addr string
	}{
		{nil, nil, ":3000"},
		{[]string{"--port", "8080"}, nil, ":8080"},
		{[]string{"--port=0", "--host", "127.0.0.1"}, nil, "127.0.0.1:0"},
		{[]string{"-host", "::1", "-port", "9"}, nil, "[::1]:9"},
		{[]string{"-addr", "127.0.0.1:4000"}, nil, "127.0.0.1:4000"},
		{nil, map[string]string{"LITECHAT_PORT": "5000", "LITECHAT_HOST": "0.0.0.0"}, "0.0.0.0:5000"},
		{nil, map[string]string{"LITECHAT_ADDR": ":6000"}, ":6000"},                        // v0.1 variable
		{[]string{"--port", "7000"}, map[string]string{"LITECHAT_ADDR": ":6000"}, ":7000"}, // flag wins
		{[]string{"--port", "7001"}, map[string]string{"LITECHAT_PORT": "6001"}, ":7001"},  // flag wins
	}
	for _, c := range cases {
		for _, k := range []string{"LITECHAT_PORT", "LITECHAT_HOST", "LITECHAT_ADDR"} {
			t.Setenv(k, c.env[k])
		}
		got, err := parseConfig(c.args)
		if err != nil || got.addr != c.addr {
			t.Errorf("%v %v: addr %q, err %v; want %q", c.args, c.env, got.addr, err, c.addr)
		}
	}

	t.Setenv("LITECHAT_PASSWORD", "from-env")
	t.Setenv("LITECHAT_MCP_STDIO", "")
	c, _ := parseConfig([]string{"--data", "/x"})
	if c.password != "from-env" || c.dataDir != "/x" || c.allowLocalMCP {
		t.Errorf("env fallbacks: %+v", c)
	}
	c, _ = parseConfig([]string{"--password", "flag", "--allow-local-mcp"})
	if c.password != "flag" || !c.allowLocalMCP {
		t.Errorf("flags: %+v", c)
	}
	for _, bad := range [][]string{{"--port", "x"}, {"--port", "70000"}, {"stray"}, {"--nope"}} {
		if _, err := parseConfig(bad); err == nil {
			t.Errorf("%v: want error", bad)
		}
	}
}
