package main

import "log"

func main() {

	// Passing any value here indicates the server has shutdown
	server, err := NewServer("server_test", "server.jar")
	if err != nil {
		log.Fatal(err)
	}

	err = server.Run()
	if err != nil {
		log.Fatal(err)
	}
}
