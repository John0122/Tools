package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInvokeMakesRealRequests(t *testing.T) {
	for _, method := range []string{"TRACE", "CONNECT"} {
		t.Run(method, func(t *testing.T) {
			requests := make(chan *http.Request, 1)
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.Clone(r.Context())
				w.Header().Set("X-Target-Proof", "real-network-response")
				NewHandler("").ServeHTTP(w, r)
			}))
			defer target.Close()
			request := httptest.NewRequest("POST", "/api/invoke", strings.NewReader(`{"method":"`+method+`"}`))
			request.Host = "untrusted.example:1234"
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer secret")
			request.Header.Set("Cookie", "session=secret")
			recorder := httptest.NewRecorder()
			NewHandler(target.URL).ServeHTTP(recorder, request)
			var result struct {
				Via, Method, Path, Body string
				Status                  int
				Headers                 http.Header
			}
			if recorder.Code != 200 || json.Unmarshal(recorder.Body.Bytes(), &result) != nil {
				t.Fatalf("relay failed: %d %s", recorder.Code, recorder.Body.String())
			}
			wantStatus := 200
			if method == "CONNECT" {
				wantStatus = 501
			}
			path := "/api/" + strings.ToLower(method)
			if result.Via != "backend" || result.Method != method || result.Path != path || result.Status != wantStatus || result.Headers.Get("X-Target-Proof") != "real-network-response" {
				t.Fatalf("relay did not preserve response: %+v", result)
			}
			var body struct{ Method, Path, Description string }
			if json.Unmarshal([]byte(result.Body), &body) != nil || body.Method != method || body.Path != path || body.Description == "" {
				t.Fatalf("invalid target body: %s", result.Body)
			}
			select {
			case actual := <-requests:
				if actual.Method != method || actual.URL.Path != path || actual.Host != strings.TrimPrefix(target.URL, "http://") || actual.Header.Get("Authorization") != "" || actual.Header.Get("Cookie") != "" {
					t.Fatalf("unexpected outgoing request: %s %s host=%s headers=%v", actual.Method, actual.URL.Path, actual.Host, actual.Header)
				}
			default:
				t.Fatal("no real network request reached the target")
			}
		})
	}
}

func TestInvokeFailuresAndLimits(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strings.Repeat("x", 64*1024+1))
	}))
	defer target.Close()
	for _, test := range []struct {
		method, contentType, body, base string
		status                          int
	}{
		{"GET", "", "", target.URL, 405},
		{"POST", "text/plain", `{"method":"TRACE"}`, target.URL, 415},
		{"POST", "application/json", `{"method":"TRACE"}` + strings.Repeat(" ", 1024), target.URL, 400},
		{"POST", "application/json", `{"method":"TRACE"}`, target.URL, 502},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, "/api/invoke", strings.NewReader(test.body))
		request.Header.Set("Content-Type", test.contentType)
		NewHandler(test.base).ServeHTTP(recorder, request)
		if recorder.Code != test.status {
			t.Errorf("%+v: status=%d", test, recorder.Code)
		}
	}
	target.Close()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/invoke", strings.NewReader(`{"method":"TRACE"}`))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(target.URL).ServeHTTP(recorder, request)
	if recorder.Code != 502 {
		t.Fatalf("unavailable target: status=%d", recorder.Code)
	}
}

func TestInvokeDoesNotFollowRedirect(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("relay followed redirect")
	}))
	defer destination.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer target.Close()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/invoke", strings.NewReader(`{"method":"TRACE"}`))
	request.Header.Set("Content-Type", "application/json")
	NewHandler(target.URL).ServeHTTP(recorder, request)
	var result struct{ Status int }
	if json.Unmarshal(recorder.Body.Bytes(), &result) != nil || result.Status != 307 {
		t.Fatalf("redirect response not preserved: %s", recorder.Body.String())
	}
}
