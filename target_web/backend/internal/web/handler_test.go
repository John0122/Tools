package web

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 期望值独立列出，防止实现漏掉一种方法时测试随同漏掉。
var expectedMethods = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "TRACE", "CONNECT"}

func TestMethodDescriptions(t *testing.T) {
	for _, method := range expectedMethods {
		for _, path := range []string{"/api/request", "/api/" + strings.ToLower(method)} {
			t.Run(method+path, func(t *testing.T) {
				request := httptest.NewRequest(method, path, nil)
				request.Header.Set("Authorization", "Bearer do-not-echo")
				recorder := httptest.NewRecorder()
				NewHandler("").ServeHTTP(recorder, request)
				wantStatus := http.StatusOK
				if method == "CONNECT" {
					wantStatus = http.StatusNotImplemented
				}
				if recorder.Code != wantStatus {
					t.Fatalf("status = %d, want %d: %s", recorder.Code, wantStatus, recorder.Body.String())
				}
				if method == "HEAD" {
					if recorder.Body.Len() != 0 || recorder.Header().Get("X-Request-Description") == "" {
						t.Fatal("HEAD must describe itself in headers without a body")
					}
					return
				}
				var body struct {
					Service     string `json:"service"`
					Method      string `json:"method"`
					Path        string `json:"path"`
					Description string `json:"description"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Service != "target_web" || body.Method != method || body.Path != path || body.Description == "" {
					t.Fatalf("unexpected response: %+v", body)
				}
				if strings.Contains(recorder.Body.String(), "do-not-echo") {
					t.Fatal("description endpoint echoed credentials")
				}
			})
		}
	}
}

func TestCatalogAndOptions(t *testing.T) {
	handler := NewHandler("")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api", nil))
	var catalog struct {
		Methods []struct {
			Method      string `json:"method"`
			Path        string `json:"path"`
			Description string `json:"description"`
			BrowserMode string `json:"browser_mode"`
		} `json:"methods"`
	}
	if recorder.Code != 200 || json.Unmarshal(recorder.Body.Bytes(), &catalog) != nil || len(catalog.Methods) != 9 {
		t.Fatalf("invalid method catalog: %s", recorder.Body.String())
	}
	seen := make(map[string]bool)
	for _, entry := range catalog.Methods {
		mode := "direct"
		if entry.Method == "TRACE" || entry.Method == "CONNECT" {
			mode = "relay"
		}
		if entry.BrowserMode != mode {
			t.Errorf("%s browser_mode = %q, want %q", entry.Method, entry.BrowserMode, mode)
		}
		if entry.Path != "/api/"+strings.ToLower(entry.Method) || entry.Description == "" {
			t.Fatalf("invalid catalog entry: %+v", entry)
		}
		seen[entry.Method] = true
	}
	for _, method := range expectedMethods {
		if !seen[method] {
			t.Errorf("missing catalog entry: %s", method)
		}
	}
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("OPTIONS", "/api/request", nil))
	if recorder.Code != 200 || recorder.Header().Get("Allow") != strings.Join(expectedMethods, ", ") {
		t.Fatalf("OPTIONS: status=%d Allow=%q", recorder.Code, recorder.Header().Get("Allow"))
	}
}

func TestRoutingErrors(t *testing.T) {
	for _, test := range []struct {
		method string
		path   string
		status int
		allow  string
	}{
		{"BREW", "/api/request", 405, strings.Join(expectedMethods, ", ")},
		{"GET", "/api/post", 405, "POST, OPTIONS"},
		{"POST", "/api", 405, "GET, HEAD, OPTIONS"},
		{"GET", "/api/missing", 404, ""},
		{"POST", "/healthz", 405, "GET, HEAD, OPTIONS"},
		{"POST", "/", 404, ""},
	} {
		t.Run(test.method+test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			NewHandler("").ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			if recorder.Code != test.status || recorder.Header().Get("Allow") != test.allow {
				t.Fatalf("status=%d Allow=%q, want %d %q", recorder.Code, recorder.Header().Get("Allow"), test.status, test.allow)
			}
		})
	}
}

func TestBackendDoesNotServeFiles(t *testing.T) {
	for _, path := range []string{"/", "/css/style.css", "/js/app.js", "/backend/go.mod", "/certs/privkey.pem", "/missing"} {
		recorder := httptest.NewRecorder()
		NewHandler("").ServeHTTP(recorder, httptest.NewRequest("GET", path, nil))
		if recorder.Code != 404 {
			t.Errorf("%s status = %d, want 404", path, recorder.Code)
		}
	}
}

func TestInvokeRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{`{}`, `{"method":"GET"}`, `{"method":"trace"}`, `{"method":"TRACE","url":"http://example.test"}`, `{"method":"TRACE"} {}`, `null`, `broken`} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/api/invoke", strings.NewReader(input))
		request.Header.Set("Content-Type", "application/json")
		NewHandler("").ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("input=%q: status=%d, want 400", input, recorder.Code)
		}
	}
}

func TestHealthThroughHTTPAndHTTPS(t *testing.T) {
	for _, secure := range []bool{false, true} {
		var server *httptest.Server
		scheme := "http"
		if secure {
			server = httptest.NewTLSServer(NewHandler(""))
			scheme = "https"
		} else {
			server = httptest.NewServer(NewHandler(""))
		}
		t.Cleanup(server.Close)
		response, err := server.Client().Get(server.URL + "/healthz")
		if err != nil {
			t.Fatal(err)
		}
		var body struct{ Status, Service, Scheme string }
		err = json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || body.Status != "ok" || body.Service != "target_web" || body.Scheme != scheme {
			t.Fatalf("health response: %+v, err=%v", body, err)
		}
	}
}

func TestConnectAuthorityForm(t *testing.T) {
	// CONNECT 的标准请求目标是 host:port，而非 /api/connect，需走真实 HTTP 解析。
	server := httptest.NewServer(NewHandler(""))
	defer server.Close()
	connection, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	fmt.Fprint(connection, "CONNECT example.test:443 HTTP/1.1\r\nHost: example.test:443\r\nConnection: close\r\n\r\n")
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: "CONNECT"})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != 501 || !strings.Contains(string(body), "CONNECT") {
		t.Fatalf("CONNECT status=%d body=%s err=%v", response.StatusCode, body, err)
	}
}
