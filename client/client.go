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

	resp, err = http.Get(fmt.Sprint("http://", *host, ":", *basePort+1))
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Received unexpected knock error: %v", resp)
	}

	log.Println("Received expected knock success")

	log.Println("Calling server again")

	resp, err = http.Get(fmt.Sprint("http://", *host, ":", *basePort))
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Received unexpected error: %v", resp)
	}

	log.Println("Received expected success")

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Received response: %+v\n", string(body))
}
