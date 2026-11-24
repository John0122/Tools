package linklab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// 以查询结果而非固定成功文案验证 GraphQL，未知字段必须失败。
func TestGraphQLQueries(t *testing.T) {
	h, _ := NewHandler(Config{ExternalHost: "pz-target-web.com"})
	defer h.Close()
	for _, tc := range []struct {
		query  string
		status int
		part   string
	}{
		{`{"query":"query($id:ID!){article(id:$id){id title} echo(message:\"实际输入\")}","variables":{"id":"intro"}}`, 200, "实际输入"},
		{`{"query":"{doesNotExist}"}`, 400, "errors"},
		{`{"query":"query($id:ID!){article(id:$id){id}}"}`, 400, "errors"},
		{`{"query":"{article(id:\"unknown\"){id}}"}`, 200, `"article":null`},
	} {
		r := httptest.NewRequest("POST", "/lab/graphql", strings.NewReader(tc.query))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status || !json.Valid(w.Body.Bytes()) || !strings.Contains(w.Body.String(), tc.part) {
			t.Fatalf("graphql: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestWebSocketEchoOriginAndShutdown(t *testing.T) {
	h, _ := NewHandler(Config{ExternalHost: "pz-target-web.com"})
	defer h.Close()
	s := httptest.NewServer(h)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	address := "ws" + strings.TrimPrefix(s.URL, "http") + "/lab/ws"
	c, resp, err := websocket.Dial(ctx, address, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://attacker.test"}}})
	if err == nil {
		c.CloseNow()
		t.Fatal("bad Origin accepted")
	}
	if resp == nil || resp.StatusCode != 403 {
		t.Fatalf("bad Origin response %v %v", resp, err)
	}
	// 本机协议测试仍走临时监听器，浏览器来源沿用部署的 HTTPS 入口。
	c, _, err = websocket.Dial(ctx, address, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://localhost:38443"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if err = c.Write(ctx, websocket.MessageText, []byte("真实消息")); err != nil {
		t.Fatal(err)
	}
	typ, body, err := c.Read(ctx)
	if err != nil || typ != websocket.MessageText || string(body) != "真实消息" {
		t.Fatalf("echo: %s %v", body, err)
	}
	// Ping 本身不读取控制帧；客户端必须同时读取才能接收 pong。
	closed := make(chan error, 1)
	go func() { _, _, readErr := c.Read(ctx); closed <- readErr }()
	if err = c.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	h.Close()
	if err = <-closed; err == nil {
		t.Fatal("connection survived server close")
	}
}
