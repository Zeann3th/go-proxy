package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

func main() {
	config := NewConfig("./application.yaml")

	balancer := NewProxyBalancer(config.Proxies)

	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			picked := balancer.Next()
			log.Printf("Forwarding HTTP request to: %s", picked.Host)
			return picked, nil
		},
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   false,
		TLSNextProto:        make(map[string]func(string, *tls.Conn) http.RoundTripper),
	}

	server := &ProxyServer{
		balancer:  balancer,
		transport: transport,
	}

	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	fmt.Printf("Load Balancing Proxy listening on %s with %d upstream proxies\n", addr, len(balancer.proxies))

	log.Fatal(http.ListenAndServe(addr, server))
}
