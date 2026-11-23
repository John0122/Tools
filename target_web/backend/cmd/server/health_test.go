package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 验证真实网络响应，不能仅凭二进制存在或端口可连接判定服务健康。
func TestCheckHealth(t *testing.T) {
	for _, test := range []struct {
		status  int
		body    string
		healthy bool
	}{
		{200, `{"status":"ok","service":"target_web"}`, true},
		{503, `{"status":"ok","service":"target_web"}`, false},
		{200, `{"status":"starting","service":"target_web"}`, false},
		{200, `{"status":"ok","service":"other"}`, false},
		{200, `invalid json`, false},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/healthz" {
				t.Errorf("unexpected probe: %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(test.status)
			_, _ = w.Write([]byte(test.body))
		}))
		err := checkHealth(server.URL + "/healthz")
		server.Close()
		if (err == nil) != test.healthy {
			t.Errorf("status=%d body=%s err=%v", test.status, test.body, err)
		}
		if err := checkHealth(server.URL + "/healthz"); err == nil {
			t.Error("closed listener reported healthy")
		}
	}
}
