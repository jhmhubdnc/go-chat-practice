package main

import "chat_server_go/network"

func main() {
	n := network.NewServer()
	n.StartServer()
}
