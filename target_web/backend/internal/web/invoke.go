package web

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"time"
)

func newInvoker(selfURL string) http.Handler {
	client := &http.Client{
		Timeout: 5 * time.Second,
		// 只请求自己的监听地址，不使用环境代理、不跟随重定向。
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Allow", "POST, OPTIONS")
			writeJSON(w, r, http.StatusOK, map[string]string{"description": "POST 一个 method 字段，后端仅代发 TRACE 或 CONNECT。"})
			return
		}
		if r.Method != http.MethodPost {
			methodNotAllowed(w, r, "POST, OPTIONS")
			return
		}
		contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || contentType != "application/json" {
			writeJSON(w, r, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type 必须为 application/json"})
			return
		}
		var input struct {
			Method string `json:"method"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, r, http.StatusBadRequest, map[string]string{"error": "请求必须是仅含 method 字段的 JSON 对象，最多 1024 字节"})
			return
		}
		if decoder.Decode(new(any)) != io.EOF || (input.Method != http.MethodTrace && input.Method != http.MethodConnect) {
			writeJSON(w, r, http.StatusBadRequest, map[string]string{"error": "只支持 TRACE、CONNECT；不接受目标地址、请求头或其他字段"})
			return
		}
		path := "/api/trace"
		if input.Method == http.MethodConnect {
			path = "/api/connect"
		}
		request, err := http.NewRequestWithContext(r.Context(), input.Method, selfURL+path, nil)
		if err != nil {
			writeJSON(w, r, http.StatusBadGateway, map[string]string{"error": "无法建立内部请求"})
			return
		}
		// 不复制浏览器的 Authorization、Cookie、Host 等信息。
		response, err := client.Do(request)
		if err != nil {
			writeJSON(w, r, http.StatusBadGateway, map[string]string{"error": "后端代发请求失败或超时"})
			return
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 64*1024+1))
		if err != nil || len(body) > 64*1024 {
			writeJSON(w, r, http.StatusBadGateway, map[string]string{"error": "无法读取内部响应"})
			return
		}
		writeJSON(w, r, http.StatusOK, struct {
			Via     string      `json:"via"`
			Method  string      `json:"method"`
			Path    string      `json:"path"`
			Status  int         `json:"status"`
			Headers http.Header `json:"headers"`
			Body    string      `json:"body"`
		}{"backend", input.Method, path, response.StatusCode, response.Header, string(body)})
	})
}
