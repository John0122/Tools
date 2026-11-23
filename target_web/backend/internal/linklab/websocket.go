package linklab

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

func (h *Handler) socket(w http.ResponseWriter, r *http.Request) {
	// Origin 已由公共入口按协议、主机和端口精确校验；此处避免库再次仅按 Host 拒绝跨站样例。
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		c.CloseNow()
		return
	}
	h.connections[c] = struct{}{}
	h.mu.Unlock()
	defer func() { h.mu.Lock(); delete(h.connections, c); h.mu.Unlock(); c.CloseNow() }()
	c.SetReadLimit(4096)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	// 读取循环负责处理控制帧；心跳不产生浏览器业务消息。
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ping, cancelPing := context.WithTimeout(ctx, 5*time.Second)
				err := c.Ping(ping)
				cancelPing()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	for {
		typ, body, err := c.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			_ = c.Close(websocket.StatusUnsupportedData, "本样例仅接收文本")
			return
		}
		write, cancelWrite := context.WithTimeout(ctx, 5*time.Second)
		err = c.Write(write, typ, body)
		cancelWrite()
		if err != nil {
			return
		}
	}
}
