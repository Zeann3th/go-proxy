package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type ProxyBalancer struct {
	proxies []*url.URL
	counter uint64
}

type ProxyServer struct {
	balancer  *ProxyBalancer
	transport *http.Transport
}

func NewProxyBalancer(proxies []ProxyConfig) *ProxyBalancer {
	parsedProxies := []*url.URL{}

	for _, proxy := range proxies {
		rawURL := proxy.URL

		if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
			rawURL = "http://" + rawURL
		}

		proxyUrl, err := url.Parse(rawURL)
		if err != nil {
			log.Printf("Skipping invalid proxy %v: %v", proxy, err)
			continue
		}
		parsedProxies = append(parsedProxies, proxyUrl)
	}
	return &ProxyBalancer{proxies: parsedProxies}
}

func (b *ProxyBalancer) Next() *url.URL {
	n := atomic.AddUint64(&b.counter, 1)
	return b.proxies[(n-1)%uint64(len(b.proxies))]
}

func (p *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleTunnel(w, r)
	} else {
		p.handleHTTP(w, r)
	}
}

func (p *ProxyServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	outReq, _ := http.NewRequest(r.Method, r.URL.String(), r.Body)

	for k, v := range r.Header {
		for _, vv := range v {
			outReq.Header.Add(k, vv)
		}
	}

	client := &http.Client{Transport: p.transport}
	resp, err := client.Do(outReq)
	if err != nil {
		message := fmt.Sprintf("Proxy Error: %v", err.Error())
		http.Error(w, message, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *ProxyServer) handleTunnel(w http.ResponseWriter, r *http.Request) {
	upstreamProxy := p.balancer.Next()
	log.Printf("Balancing HTTPS tunnel to: %s", upstreamProxy.Host)

	upstreamConn, err := net.DialTimeout("tcp", upstreamProxy.Host, 10*time.Second)
	if err != nil {
		http.Error(w, "Proxy Unreachable", http.StatusBadGateway)
		return
	}
	defer upstreamConn.Close()

	authHeader := ""
	if pass, ok := upstreamProxy.User.Password(); ok {
		auth := upstreamProxy.User.Username() + ":" + pass
		basicAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		authHeader = "Proxy-Authorization: Basic " + basicAuth + "\r\n"
	}

	fmt.Fprintf(upstreamConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n%s\r\n", r.Host, r.Host, authHeader)

	buf := make([]byte, 1024)
	n, err := upstreamConn.Read(buf)
	if err != nil {
		http.Error(w, "Proxy Read Error", http.StatusBadGateway)
		return
	}

	if string(buf[:10]) != "HTTP/1.1 2" && string(buf[:10]) != "HTTP/1.0 2" {
		log.Printf("Upstream failed: %s", string(buf[:n]))
		http.Error(w, "Upstream Rejected", http.StatusBadGateway)
		return
	}

	hijacker, _ := w.(http.Hijacker)
	clientConn, _, _ := hijacker.Hijack()
	defer clientConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	go io.Copy(upstreamConn, clientConn)
	io.Copy(clientConn, upstreamConn)
}
