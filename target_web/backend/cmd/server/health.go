package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// checkHealth 供无 shell 的运行镜像检查实际 HTTP 服务。
func checkHealth(url string) error {
	client := &http.Client{
		Timeout:       2 * time.Second,
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	var result struct{ Status, Service string }
	if err := json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&result); err != nil {
		return err
	}
	if result.Status != "ok" || result.Service != "target_web" {
		return fmt.Errorf("服务尚未就绪")
	}
	return nil
}
