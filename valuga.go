package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/net/proxy"
)

func handleHTTP(w http.ResponseWriter, req *http.Request, dialer proxy.Dialer) {
	tp := http.Transport{
		Dial: dialer.Dial,
	}
	resp, err := tp.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func handleTunnel(w http.ResponseWriter, req *http.Request, dialer proxy.Dialer) {
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
	dstConn, err := dialer.Dial("tcp", req.Host)
	if err != nil {
		srcConn.Close()
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

func serveHttp(socksAddr string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		d := &net.Dialer{
			Timeout: 10 * time.Second,
		}
		dialer, _ := proxy.SOCKS5("tcp", socksAddr, nil, d)

		if req.Method == "CONNECT" {
			handleTunnel(w, req, dialer)
		} else {
			handleHTTP(w, req, dialer)
		}
	}
}

func main() {
	var httpListenPort = flag.Int("p", 1081, "HTTP listen port")
	var httpListenHost = flag.String("h", "127.0.0.1", "HTTP listen host")
	var socks5Addr = flag.String("s", "127.0.0.1:1080", "Upstream socks5 address")
	flag.Parse()
	var httpListenAddr = *httpListenHost + ":" + strconv.Itoa(*httpListenPort)
	log.Println("Listening on ", httpListenAddr)
	log.Println("Connect to Socks5 Proxy on ", *socks5Addr)
	http.ListenAndServe(httpListenAddr, http.HandlerFunc(serveHttp(*socks5Addr)))
}
