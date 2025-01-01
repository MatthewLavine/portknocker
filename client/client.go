package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
)

var (
	host     = flag.String("host", "localhost", "The host to connect to")
	basePort = flag.Int("basePort", 8080, "The base port to use for the server")
)

func main() {
	log.Println("Calling server without knocking")

	resp, err := http.Get(fmt.Sprint("http://", *host, ":", *basePort))
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode == http.StatusOK {
		log.Fatal("Received unexpected success")
	}

	log.Println("Received expected 403")

	log.Println("Knocking server")

	knock(8081)
	knock(8082)
	knock(8083)

	log.Println("Calling server again")

	log.Printf("Received response: %+v\n", get(8080))
}

func get(port int) string {
	resp, err := http.Get(fmt.Sprint("http://", *host, ":", port))
	if err != nil {
		log.Fatalf("error in http.Get(%d): %v", port, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("error in io.ReadAll(%d): %v", port, err)
	}
	return string(body)
}

func knock(port int) {
	_, err := http.Get(fmt.Sprint("http://", *host, ":", port))
	if err != nil {
		log.Fatalf("error in http.Get(%d): %v", port, err)
	}
}
