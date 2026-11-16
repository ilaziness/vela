package ssh

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestProxyConfigEnabledAndCacheKey(t *testing.T) {
	if (ProxyConfig{}).Enabled() {
		t.Fatal("empty should be disabled")
	}
	if (ProxyConfig{}).cacheKey() != "direct" {
		t.Fatal(ProxyConfig{}.cacheKey())
	}
	p := ProxyConfig{Type: "HTTP", Host: "p.example", Port: 3128, User: "u", Password: "secret"}
	if !p.Enabled() {
		t.Fatal("expected enabled")
	}
	if p.cacheKey() != "http|p.example|3128|u" {
		t.Fatal(p.cacheKey())
	}
	if (ProxyConfig{Type: "http", Host: "h", Port: 70000}).Enabled() {
		t.Fatal("port out of range should be disabled")
	}
}

func TestDialHTTPConnect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	targetLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer targetLn.Close()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				req, err := http.ReadRequest(br)
				if err != nil {
					return
				}
				if req.Method != http.MethodConnect {
					return
				}
				auth := req.Header.Get("Proxy-Authorization")
				if !strings.HasPrefix(auth, "Basic ") {
					_ = (&http.Response{StatusCode: http.StatusProxyAuthRequired, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}).Write(c)
					return
				}
				_, _ = io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
				upstream, err := net.Dial("tcp", targetLn.Addr().String())
				if err != nil {
					return
				}
				defer upstream.Close()
				errCh := make(chan error, 2)
				go func() { _, err := io.Copy(upstream, br); errCh <- err }()
				go func() { _, err := io.Copy(c, upstream); errCh <- err }()
				<-errCh // copy errors are expected on teardown; we only wait for the first direction to end
			}(c)
		}
	}()

	go func() {
		c, err := targetLn.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(c, buf); err != nil {
			return
		}
		_, _ = c.Write([]byte("pong"))
	}()

	proxyHost, proxyPortStr, _ := net.SplitHostPort(ln.Addr().String())
	var proxyPort int
	_, _ = fmt.Sscanf(proxyPortStr, "%d", &proxyPort)
	conn, err := dialTCP(targetLn.Addr().String(), 3*time.Second, ProxyConfig{
		Type: "http", Host: proxyHost, Port: proxyPort, User: "u", Password: "p",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "pong" {
		t.Fatalf("got %q", buf)
	}
}

func TestDialHTTPConnectReject(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		br := bufio.NewReader(c)
		_, _ = http.ReadRequest(br)
		_, _ = io.WriteString(c, "HTTP/1.1 403 Forbidden\r\n\r\n")
	}()
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)
	_, err = dialTCP("example.com:22", time.Second, ProxyConfig{Type: "http", Host: host, Port: port})
	if err == nil {
		t.Fatal("expected error")
	}
}
