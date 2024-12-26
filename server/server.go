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
	accessDuration = flag.Duration("accessDuration", 5*time.Second, "The duration to allow access after a successful knock")
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
	logAllowedPeers()
	gracefulshutdown.AddShutdownHandler(func() error {
		log.Println("Shutting down port knock server...")
		defer log.Println("Port knock server shut down.")
		return nil
	})
	peerManagerContext, peerManagerCancel := context.WithCancel(ctx)
	gracefulshutdown.AddShutdownHandler(func() error {
		log.Println("Shutting down peer manager...")
		defer log.Println("Peer manager shut down.")
		peerManagerCancel()
		return nil
	})
	startPeerManager(peerManagerContext)
	startBaseServer(ctx)
	startKnockServers(ctx)
	gracefulshutdown.WaitForShutdown()
}

func startPeerManager(ctx context.Context) {
	go func() {
		log.Println("Starting peer manager...")
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ctx.Done():
				log.Println("Shutting down peer manager...")
				return
			case <-ticker.C:
				for i := 0; i < len(allowedPeers); i++ {
					if time.Now().After(allowedPeers[i].end) {
						log.Printf("Removing expired peer %s\n", allowedPeers[i].ip)
						allowedPeers = append(allowedPeers[:i], allowedPeers[i+1:]...)
						i--
					}
				}
			}
		}
	}()
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
			http.Error(w, "Access denied!", http.StatusForbidden)
			return
		}

		w.Write([]byte("Access granted!"))
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
