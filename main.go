package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	config := NewConfig("./application.yaml")

	balancer := NewProxyBalancer(config.Proxies)

	server := &ProxyServer{
		balancer: balancer,
	}

	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	fmt.Printf("Load Balancing Proxy listening on %s with %d upstream proxies\n", addr, len(balancer.upstreams))

	log.Fatal(http.ListenAndServe(addr, server))
}
