package main

import (
	"net"
	"net/http"

	"github.com/frantjc/sindri"
)

func main() {
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	http.Serve(lis, sindri.Handler())
}
