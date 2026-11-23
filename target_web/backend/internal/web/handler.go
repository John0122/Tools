package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

type methodInfo struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
	BrowserMode string `json:"browser_mode"`
}

// 方法目录同时用于发现接口与生成响应，保持说明与实现一致。
var methods = []methodInfo{
	{http.MethodGet, "/api/get", "GET 请求用于获取资源或查询信息；当前接口返回一段描述信息。", "direct"},
	{http.MethodHead, "/api/head", "HEAD 请求只返回响应头，不返回正文；描述可查看 X-Request-Description 响应头。", "direct"},
	{http.MethodPost, "/api/post", "POST 请求用于提交数据或创建资源；当前接口仅返回说明，不保存数据。", "direct"},
	{http.MethodPut, "/api/put", "PUT 请求用于整体替换资源；当前接口仅返回说明，不修改数据。", "direct"},
	{http.MethodPatch, "/api/patch", "PATCH 请求用于局部更新资源；当前接口仅返回说明，不修改数据。", "direct"},
	{http.MethodDelete, "/api/delete", "DELETE 请求用于删除资源；当前接口仅返回说明，不删除数据。", "direct"},
	{http.MethodOptions, "/api/options", "OPTIONS 请求用于查询接口支持的方法；请查看 Allow 响应头。", "direct"},
	{http.MethodTrace, "/api/trace", "TRACE 请求用于诊断请求链路；当前接口仅返回说明，不回显请求头或认证信息。", "relay"},
	{http.MethodConnect, "/api/connect", "CONNECT 请求用于建立代理隧道；当前接口以 501 返回说明，不建立隧道或转发连接。", "relay"},
}

// NewHandler 只提供 API。selfURL 由服务实际监听地址生成，不接受客户端指定目标。
func NewHandler(selfURL string) http.Handler {
	invoke := newInvoker(selfURL)
	byMethod := make(map[string]methodInfo, len(methods))
	byPath := make(map[string]methodInfo, len(methods))
	allowed := make([]string, 0, len(methods))
	for _, entry := range methods {
		byMethod[entry.Method] = entry
		byPath[entry.Path] = entry
		allowed = append(allowed, entry.Method)
	}
	allMethods := strings.Join(allowed, ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/api/invoke":
			invoke.ServeHTTP(w, r)
		case r.Method == http.MethodConnect && path == "" && r.URL.Host != "":
			// 标准 CONNECT 使用 host:port 作为目标；只描述，不执行代理转发。
			describe(w, r, byMethod[http.MethodConnect])
		case r.Method == http.MethodOptions && r.RequestURI == "*":
			w.Header().Set("Allow", allMethods)
			describe(w, r, byMethod[http.MethodOptions])
		case path == "/api" || path == "/api/" || path == "/healthz":
			if r.Method == http.MethodOptions {
				w.Header().Set("Allow", "GET, HEAD, OPTIONS")
				describe(w, r, byMethod[http.MethodOptions])
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				methodNotAllowed(w, r, "GET, HEAD, OPTIONS")
				return
			}
			if path == "/healthz" {
				scheme := "http"
				// 容器只在内部网络监听，入口 Nginx 会覆盖此头。
				if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
					scheme = "https"
				}
				writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok", "service": "target_web", "scheme": scheme})
				return
			}
			writeJSON(w, r, http.StatusOK, struct {
				Service     string       `json:"service"`
				Description string       `json:"description"`
				Endpoint    string       `json:"endpoint"`
				Methods     []methodInfo `json:"methods"`
			}{"target_web", "HTTP 请求方法演示接口，仅输出描述信息。", "/api/request", methods})
		case path == "/api/request":
			entry, ok := byMethod[r.Method]
			if !ok {
				methodNotAllowed(w, r, allMethods)
				return
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Allow", allMethods)
			}
			describe(w, r, entry)
		case strings.HasPrefix(path, "/api/"):
			entry, ok := byPath[path]
			if !ok {
				writeJSON(w, r, http.StatusNotFound, map[string]string{"error": "接口不存在", "path": path})
				return
			}
			allow := entry.Method
			if entry.Method == http.MethodGet {
				allow += ", HEAD"
			}
			if entry.Method != http.MethodOptions {
				allow += ", OPTIONS"
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Allow", allow)
				describe(w, r, byMethod[http.MethodOptions])
				return
			}
			if r.Method != entry.Method && !(entry.Method == http.MethodGet && r.Method == http.MethodHead) {
				methodNotAllowed(w, r, allow)
				return
			}
			describe(w, r, byMethod[r.Method])
		default:
			writeJSON(w, r, http.StatusNotFound, map[string]string{"error": "接口不存在", "path": path})
		}
	})
}

func describe(w http.ResponseWriter, r *http.Request, entry methodInfo) {
	status := http.StatusOK
	if r.Method == http.MethodConnect {
		// CONNECT 的 2xx 意味着已建立隧道，不能用 200 冒充普通 JSON 描述接口。
		status = http.StatusNotImplemented
	}
	path := r.URL.Path
	if path == "" {
		path = r.RequestURI
	}
	writeJSON(w, r, status, struct {
		Service     string `json:"service"`
		Method      string `json:"method"`
		Path        string `json:"path"`
		Description string `json:"description"`
	}{"target_web", r.Method, path, entry.Description})
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request, allowed string) {
	w.Header().Set("Allow", allowed)
	writeJSON(w, r, http.StatusMethodNotAllowed, map[string]string{"error": "此接口不支持该请求方法", "method": r.Method})
}

// HEAD 不写正文；描述使用 ASCII 响应头，避免非 ASCII 头值的客户端兼容问题。
func writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.Header().Set("X-Request-Description", "HEAD returns response headers without a response body.")
		w.WriteHeader(status)
		return
	}
	w.WriteHeader(status)
	// 编码的值均为内部字符串/结构；连接中断时交由 HTTP 服务关闭连接。
	_ = json.NewEncoder(w).Encode(value)
}
