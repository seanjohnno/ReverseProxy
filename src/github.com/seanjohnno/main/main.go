package main

import (
	"fmt"
	"github.com/seanjohnno/reverseproxy"
)

func main() {
	sb, err := reverseproxy.LoadConfigFromFile("/home/sean/Development/go/ReverseProxy/proxy.config")
	if err != nil {
		panic(err)
	}
	for _, val := range sb {
		for _, h := range val.Hostnames {
			fmt.Println(h)
		}
	}
}
