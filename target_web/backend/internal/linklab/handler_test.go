package linklab

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// 实际 HTTP 响应验证：错误状态、输入拒绝和跳转目标必须对扫描器可见。
func TestLabHTTP(t *testing.T) {
	h, err := NewHandler(Config{ExternalHost: "pz-target-web.com", PrimaryHost: "192.168.5.30"})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	tests := []struct {
		name, method, path, body, contentType, origin string
		status                                        int
		contains                                      string
	}{
		{"query", "GET", "/lab/rest?name=Alice", "", "", "", 200, "Alice"},
		{"submit", "POST", "/lab/rest", `{"name":"林","message":"你好"}`, "application/json", "", 200, "你好"},
		{"json-type", "POST", "/lab/rest", `{}`, "text/plain", "", 415, "error"},
		{"json-invalid", "POST", "/lab/rest", `{"name":`, "application/json", "", 400, "error"},
		{"json-extra", "POST", "/lab/rest", `{"url":"http://other.test"}`, "application/json", "", 400, "error"},
		{"json-trailing", "POST", "/lab/rest", `{} {}`, "application/json", "", 400, "error"},
		{"too-large", "POST", "/lab/rest", `{"message":"` + strings.Repeat("x", 17000) + `"}`, "application/json", "", 413, "error"},
		{"form", "POST", "/lab/form", "name=Alice&message=hello", "application/x-www-form-urlencoded", "", 200, "Alice"},
		{"form-get", "GET", "/lab/form?name=Alice&message=hello", "", "", "", 200, "hello"},
		{"form-empty", "POST", "/lab/form", "name=&message=hello", "application/x-www-form-urlencoded", "", 400, "error"},
		{"missing", "GET", "/lab/status/404", "", "", "", 404, "预置"},
		{"gone", "GET", "/lab/status/410", "", "", "", 410, "预置"},
		{"unknown", "GET", "/lab/unknown", "", "", "", 404, "error"},
		{"wrong-method", "DELETE", "/lab/rest", "", "", "", 405, "error"},
		{"bad-origin", "GET", "/lab/rest", "", "", "https://attacker.test", 403, "error"},
		{"origin-null", "GET", "/lab/rest", "", "", "null", 403, "error"},
		{"allowed-origin", "GET", "/lab/rest", "", "", "https://192.168.5.30:38443", 200, "target_web"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
				t.Fatalf("got %d %s", w.Code, w.Body.String())
			}
			if tc.status == 403 && w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unapproved origin allowed")
			}
			if tc.name == "allowed-origin" && w.Header().Get("Access-Control-Allow-Origin") != tc.origin {
				t.Fatal("CORS missing")
			}
		})
	}
}

func TestLabRedirectAndPreflight(t *testing.T) {
	h, _ := NewHandler(Config{ExternalHost: "pz-target-web.com", PrimaryHost: "127.0.0.1"})
	defer h.Close()
	for _, code := range []string{"301", "302", "303", "307", "308"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/lab/redirect/"+code+"/external/ok?url=https://attacker.test", nil))
		if !strings.HasPrefix(w.Header().Get("Location"), "https://pz-target-web.com:38443/fixtures/landing.html") || strings.Contains(w.Header().Get("Location"), "attacker") {
			t.Fatalf("redirect = %d %s", w.Code, w.Header().Get("Location"))
		}
	}
	r := httptest.NewRequest("OPTIONS", "/lab/rest", nil)
	r.Header.Set("Origin", "http://localhost:38080")
	r.Header.Set("Access-Control-Request-Method", "POST")
	r.Header.Set("Access-Control-Request-Headers", "content-type")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:38080" {
		t.Fatalf("preflight %d %v", w.Code, w.Header())
	}
	r.Header.Set("Access-Control-Request-Method", "DELETE")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 405 {
		t.Fatalf("invalid preflight %d", w.Code)
	}
	for _, path := range []string{"/lab/rest", "/lab/status/410"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("HEAD", path, nil))
		if w.Body.Len() != 0 {
			t.Fatal("HEAD wrote body")
		}
	}
}

func TestLabConfigAndReturnedInput(t *testing.T) {
	for _, host := range []string{"https://bad.test", "bad.test/abc", "*.test", "bad\n.test", "localhost", "127.0.0.1"} {
		if h, err := NewHandler(Config{ExternalHost: host}); err == nil {
			h.Close()
			t.Fatalf("accepted %q", host)
		}
	}
	h, err := NewHandler(Config{ExternalHost: "pz-target-web.com", PrimaryHost: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/lab/rest?name=%3Cscript%3E", nil))
	var body struct {
		Input struct {
			Name string `json:"name"`
		} `json:"input"`
	}
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Input.Name != "<script>" {
		t.Fatalf("actual input lost: %s", w.Body.String())
	}
}
