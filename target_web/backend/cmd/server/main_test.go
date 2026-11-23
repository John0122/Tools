package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRunRejectsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := run(context.Background(), config{httpAddr: listener.Addr().String()}); err == nil {
		t.Fatal("occupied port must fail startup")
	}
}

func TestAPIServiceAndShutdown(t *testing.T) {
	// 使用实际随机监听端口，验证代发地址也来自监听器，而非固定端口或请求 Host。
	for _, bind := range []string{"127.0.0.1:0", "0.0.0.0:0", "[::1]:0", "[::]:0"} {
		t.Run(bind, func(t *testing.T) {
			listener, err := net.Listen("tcp", bind)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- serve(ctx, listener) }()
			addr := listener.Addr().(*net.TCPAddr)
			host := addr.IP.String()
			if addr.IP.IsUnspecified() {
				host = "::1"
				if addr.IP.To4() != nil {
					host = "127.0.0.1"
				}
			}
			_, port, _ := net.SplitHostPort(listener.Addr().String())
			base := "http://" + net.JoinHostPort(host, port)
			client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
			response, err := client.Post(base+"/api/invoke", "application/json", strings.NewReader(`{"method":"CONNECT"}`))
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Method string
				Status int
			}
			err = json.NewDecoder(response.Body).Decode(&result)
			response.Body.Close()
			if err != nil || response.StatusCode != 200 || result.Method != "CONNECT" || result.Status != 501 {
				t.Fatalf("live relay response: %+v, err=%v", result, err)
			}
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("shutdown: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("listener did not stop")
			}
			connection, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 100*time.Millisecond)
			if err == nil {
				connection.Close()
				t.Fatal("listener still accepts connections")
			}
		})
	}
}
