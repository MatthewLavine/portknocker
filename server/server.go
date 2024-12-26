package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/MatthewLavine/gracefulshutdown"
)

var (
	basePort       = flag.Int("basePort", 8080, "The base port to use for the server")
	knockLength    = flag.Int("knockLength", 1, "The number of ports to knock on")
	accessDuration = flag.Duration("accessDuration", 5*time.Minute, "The duration to allow access after a successful knock")
	allowedPeers   = []*allowedPeer{}
)

type allowedPeer struct {
	ip    net.IP
	start time.Time
	end   time.Time
}

func main() {
	ctx := context.Background()
	log.Println("Starting port knock server...")
	gracefulshutdown.AddShutdownHandler(func() error {
		log.Println("Shutting down port knock server...")
		defer log.Println("Port knock server shut down.")
		return nil
	})
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
			logAllowedPeers()
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

func logAllowedPeers() {
	log.Println("Allowed peers:")
	if len(allowedPeers) == 0 {
		log.Println(" - None")
		return
	}
	for _, allowed := range allowedPeers {
		log.Printf(" - %s (Expiration in %s)\n", allowed.ip, time.Until(allowed.end).Round(time.Second))
	}
}

func allowPeer(peer net.IP) {
	if isPeerAllowed(peer) {
		log.Printf("Peer %s is already allowed\n", peer)
		logAllowedPeers()
		return
	}
	log.Printf("Allowing peer %s\n", peer)
	allowedPeers = append(allowedPeers, &allowedPeer{
		ip:    peer,
		start: time.Now(),
		end:   time.Now().Add(*accessDuration),
	})
	logAllowedPeers()
}

func isPeerAllowed(peer net.IP) bool {
	for _, allowed := range allowedPeers {
		if allowed.ip.Equal(peer) {
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
			if err == http.ErrServerClosed {
				return
			}
			log.Fatal(err)
		}
	}(s)
}
