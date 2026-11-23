package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"target_web/internal/linklab"
	"target_web/internal/web"
)

type config struct{ httpAddr string }

func main() {
	// scratch 镜像没有 shell/wget，由同一个二进制执行容器健康检查。
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := checkHealth("http://127.0.0.1:8000/healthz"); err != nil {
			log.Printf("健康检查失败：%v", err)
			os.Exit(1)
		}
		return
	}
	var configuration config
	flag.StringVar(&configuration.httpAddr, "http-addr", ":8000", "内部 API 的 HTTP 监听地址")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, configuration); err != nil {
		log.Printf("服务退出：%v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, configuration config) error {
	listener, err := net.Listen("tcp", configuration.httpAddr)
	if err != nil {
		return fmt.Errorf("监听 HTTP：%w", err)
	}
	defer listener.Close()
	return serve(ctx, listener)
}

func serve(ctx context.Context, listener net.Listener) error {
	address := listener.Addr().(*net.TCPAddr)
	ip := address.IP
	if ip.IsUnspecified() {
		ip = net.IPv6loopback
		if address.IP.To4() != nil {
			ip = net.IPv4(127, 0, 0, 1)
		}
	}
	selfURL := "http://" + net.JoinHostPort(ip.String(), fmt.Sprint(address.Port))
	handler := web.NewHandler(selfURL)
	lab, err := linklab.NewHandler(linklab.Config{ExternalHost: os.Getenv("TARGET_WEB_EXTERNAL_HOST"), PrimaryHost: os.Getenv("TARGET_WEB_HOST")})
	if err != nil {
		return err
	}
	defer lab.Close()
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/healthz" {
				log.Printf("%s %s", r.Method, r.URL.Path)
			}
			// 保留原 API 对 OPTIONS *、CONNECT authority 的处理语义。
			if strings.HasPrefix(r.URL.Path, "/lab/") {
				lab.ServeHTTP(w, r)
			} else {
				handler.ServeHTTP(w, r)
			}
		}),
		ReadHeaderTimeout:            5 * time.Second,
		ReadTimeout:                  15 * time.Second,
		WriteTimeout:                 15 * time.Second,
		IdleTimeout:                  60 * time.Second,
		DisableGeneralOptionsHandler: true,
	}
	defer server.Close()
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()
	log.Printf("target_web API 已启动：HTTP %s", listener.Addr())
	var serveError error
	select {
	case <-ctx.Done():
	case serveError = <-serverErrors:
		if errors.Is(serveError, http.ErrServerClosed) {
			serveError = nil
		}
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return errors.Join(serveError, server.Shutdown(shutdownContext))
}
