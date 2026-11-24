// Package linklab 提供外链检测用的固定演示接口。
package linklab

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/graphql-go/graphql"
)

type Config struct{ ExternalHost, PrimaryHost string }

// Handler 持有连接生命周期；主服务关闭时一并释放升级后的连接。
type Handler struct {
	external    string
	origins     map[string]bool
	schema      graphql.Schema
	mu          sync.Mutex
	closed      bool
	connections map[*websocket.Conn]struct{}
}

var dnsLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func NewHandler(c Config) (*Handler, error) {
	if c.ExternalHost == "" {
		c.ExternalHost = "pz-target-web.com"
	}
	if c.PrimaryHost == "" {
		c.PrimaryHost = "127.0.0.1"
	}
	if len(c.ExternalHost) > 253 || !strings.Contains(c.ExternalHost, ".") || net.ParseIP(c.ExternalHost) != nil {
		return nil, fmt.Errorf("外站主机名必须是独立 DNS 名称")
	}
	for _, label := range strings.Split(c.ExternalHost, ".") {
		if !dnsLabel.MatchString(label) {
			return nil, fmt.Errorf("外站主机名无效")
		}
	}
	if net.ParseIP(c.PrimaryHost) == nil {
		return nil, fmt.Errorf("主站访问地址必须是 IP")
	}
	h := &Handler{external: c.ExternalHost, origins: map[string]bool{}, connections: map[*websocket.Conn]struct{}{}}
	// 浏览器只从对外发布的 HTTPS 入口访问；容器内 HTTP 不加入跨域来源。
	for _, host := range []string{c.PrimaryHost, "127.0.0.1", "localhost", "::1", c.ExternalHost} {
		h.origins["https://"+net.JoinHostPort(host, "38443")] = true
	}
	var err error
	h.schema, err = newSchema()
	return h, err
}

func (h *Handler) Close() {
	h.mu.Lock()
	h.closed = true
	connections := make([]*websocket.Conn, 0, len(h.connections))
	for c := range h.connections {
		connections = append(connections, c)
	}
	h.mu.Unlock()
	for _, c := range connections {
		_ = c.CloseNow()
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if origin != "" && !h.origins[origin] {
		fail(w, r, 403, "来源不在测试站点范围内")
		return
	}
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Expose-Headers", "Location, Content-Disposition, X-Lab-Case")
	}
	path := r.URL.Path
	allowed := "GET, HEAD"
	switch {
	case path == "/lab/rest" || path == "/lab/form":
		allowed = "GET, HEAD, POST"
	case path == "/lab/graphql":
		allowed = "POST"
	case path == "/lab/ws":
		allowed = "GET"
	case path == "/lab/status/404" || path == "/lab/status/410" || strings.HasPrefix(path, "/lab/redirect/"):
	default:
		fail(w, r, 404, "样例接口不存在")
		return
	}
	w.Header().Set("Allow", allowed+", OPTIONS")
	method := r.Method
	if method == "OPTIONS" {
		method = r.Header.Get("Access-Control-Request-Method")
		if method == "" {
			method = strings.Split(allowed, ", ")[0]
		}
	}
	validMethod := false
	for _, item := range strings.Split(allowed, ", ") {
		if item == method {
			validMethod = true
		}
	}
	if !validMethod {
		fail(w, r, 405, "此样例不支持该方法")
		return
	}
	if r.Method == "OPTIONS" {
		for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
			if header = strings.ToLower(strings.TrimSpace(header)); header != "" && header != "content-type" {
				fail(w, r, 403, "预检包含不支持的请求头")
				return
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", allowed)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(204)
		return
	}
	switch {
	case path == "/lab/rest":
		h.rest(w, r)
	case path == "/lab/form":
		h.form(w, r)
	case path == "/lab/graphql":
		h.graphql(w, r)
	case path == "/lab/ws":
		h.socket(w, r)
	case strings.HasPrefix(path, "/lab/status/"):
		code, _ := strconv.Atoi(strings.TrimPrefix(path, "/lab/status/"))
		w.Header().Set("X-Lab-Case", "intentional-dead-link")
		fail(w, r, code, "预置死链：目标资源不存在或已移除")
	case strings.HasPrefix(path, "/lab/redirect/"):
		h.redirect(w, r)
	}
}

type input struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// 所有演示接口只返回实际收到的有限字段，不保存用户数据。
func (h *Handler) rest(w http.ResponseWriter, r *http.Request) {
	v := input{Name: r.URL.Query().Get("name"), Message: r.URL.Query().Get("message")}
	if r.Method == "POST" && !decodeJSON(w, r, &v) {
		return
	}
	if len(v.Name) > 256 || len(v.Message) > 4096 {
		fail(w, r, 400, "演示字段过长")
		return
	}
	respond(w, r, 200, map[string]any{"service": "target_web", "scenario": "rest", "method": r.Method, "input": v, "description": "真实接收请求并返回演示数据，不保存数据。"})
}

func (h *Handler) form(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var err error
	if r.Method == "POST" {
		typ, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		switch typ {
		case "multipart/form-data":
			err = r.ParseMultipartForm(16 * 1024)
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
				if len(r.MultipartForm.File) > 0 {
					fail(w, r, 400, "本样例仅接收文本字段")
					return
				}
			}
		case "application/x-www-form-urlencoded":
			err = r.ParseForm()
		default:
			fail(w, r, 415, "请使用表单编码提交")
			return
		}
	} else {
		err = r.ParseForm()
	}
	if err != nil {
		inputError(w, r, err)
		return
	}
	v := input{Name: strings.TrimSpace(r.Form.Get("name")), Message: r.Form.Get("message")}
	if v.Name == "" || len(v.Name) > 80 || len(v.Message) > 1000 {
		fail(w, r, 400, "姓名必填且不超过 80 字节，留言不超过 1000 字节")
		return
	}
	respond(w, r, 200, map[string]any{"service": "target_web", "scenario": "form", "method": r.Method, "input": v, "description": "表单已收到，数据仅用于本次响应。"})
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/lab/redirect/"), "/")
	if len(parts) != 3 {
		fail(w, r, 404, "重定向样例不存在")
		return
	}
	code, _ := strconv.Atoi(parts[0])
	if code != 301 && code != 302 && code != 303 && code != 307 && code != 308 {
		fail(w, r, 404, "重定向状态不存在")
		return
	}
	if (parts[1] != "internal" && parts[1] != "external") || (parts[2] != "ok" && parts[2] != "missing") {
		fail(w, r, 404, "重定向目标不存在")
		return
	}
	target := "/fixtures/landing.html"
	if parts[2] == "missing" {
		target = "/lab/status/404"
		if parts[1] == "external" {
			target = "/lab/status/410"
		}
	}
	if parts[1] == "external" {
		target = "https://" + net.JoinHostPort(h.external, "38443") + target
	}
	// 只允许预置路径，url/to 等查询参数不会成为跳转目标。
	if id := r.URL.Query().Get("case"); len(id) <= 80 && id != "" {
		target += "?case=" + url.QueryEscape(id)
	}
	w.Header().Set("Location", target)
	respond(w, r, code, map[string]any{"status": code, "target": target})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || typ != "application/json" {
		fail(w, r, 415, "Content-Type 必须是 application/json")
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		inputError(w, r, err)
		return false
	}
	if err = d.Decode(new(any)); err != io.EOF {
		if err == nil {
			err = errors.New("extra JSON")
		}
		inputError(w, r, err)
		return false
	}
	return true
}

func inputError(w http.ResponseWriter, r *http.Request, err error) {
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		fail(w, r, 413, "请求体不能超过 16 KiB")
		return
	}
	fail(w, r, 400, "请求内容格式不正确")
}

func fail(w http.ResponseWriter, r *http.Request, code int, message string) {
	respond(w, r, code, map[string]any{"error": message, "status": code})
}
func respond(w http.ResponseWriter, r *http.Request, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if r.Method != "HEAD" {
		_ = json.NewEncoder(w).Encode(v)
	}
}
