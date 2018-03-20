package main

import (
	"io"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

func ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	srcConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	d := &net.Dialer{
		Timeout: 10 * time.Second,
	}
	dialer, _ := proxy.SOCKS5("tcp", "127.0.0.1:1080", nil, d)
	dstConn, err := dialer.Dial("tcp", req.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	srcConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	go transfer(dstConn, srcConn)
	go transfer(srcConn, dstConn)

}

func transfer(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()

	io.Copy(dst, src)
}

func main() {
	http.ListenAndServe("127.0.0.1:8124", http.HandlerFunc(ServeHTTP))
}
