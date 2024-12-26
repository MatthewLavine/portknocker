package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/MatthewLavine/gracefulshutdown"
)

var (
	basePort    = flag.Int("basePort", 8080, "The base port to use for the server")
	knockLength = flag.Int("knockLength", 1, "The number of ports to knock on")
	allowedIps  = []net.IP{}
)

func main() {
	ctx := context.Background()
	startBaseServer(ctx)
	startKnockServers(ctx)
	gracefulshutdown.WaitForShutdown()
}

func startBaseServer(ctx context.Context) {
	startHttpServer(ctx, *basePort, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, err := getPeer(r)
		if err != nil {
			log.Printf("Error getting peer: %v\n", err)
			http.Error(w, "Error getting peer", http.StatusInternalServerError)
			return
		}
		log.Printf("Received request to %s from %s\n", r.Host, peer)

		if !isPeerAllowed(peer) {
			log.Printf("Peer %s is not allowed\n", peer)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		w.Write([]byte("Hello, World!"))
	}))
}

func startKnockServers(ctx context.Context) {
	for i := 1; i <= *knockLength; i++ {
		go func(port int) {
			startHttpServer(ctx, port, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				peer, err := getPeer(r)
				if err != nil {
					log.Printf("Error getting peer: %v\n", err)
					http.Error(w, "Error getting peer", http.StatusInternalServerError)
					return
				}
				log.Printf("Received knock request to %s from %s\n", r.Host, peer)
				allowPeer(peer)
				w.Write([]byte("Knock, knock!"))
			}))
		}(*basePort + i)
	}
}

func allowPeer(peer net.IP) {
	allowedIps = append(allowedIps, peer)
}

func isPeerAllowed(peer net.IP) bool {
	for _, allowed := range allowedIps {
		if allowed.Equal(peer) {
			return true
		}
	}
	return false
}

func getPeer(r *http.Request) (net.IP, error) {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil, err
	}
	return net.ParseIP(peer), nil
}

func startHttpServer(ctx context.Context, port int, handler http.Handler) {
	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}
	gracefulshutdown.AddShutdownHandler(func() error {
		log.Printf("Shutting down HTTP server on %d...\n", port)
		defer log.Printf("HTTP server on %d shut down.\n", port)
		return s.Shutdown(ctx)
	})
	go func(s *http.Server) {
		log.Printf("HTTP server listening on %s\n", s.Addr)
		if err := s.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}(s)
}
