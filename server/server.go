package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MatthewLavine/gracefulshutdown"
)

var (
	basePort           = flag.Int("basePort", 8080, "The base port to use for the server")
	knockLength        = flag.Int("knockLength", 3, "The number of ports to knock on")
	knockSequence      = flag.String("knockSequence", "8081,8082,8083", "The sequence of ports to knock on")
	accessDuration     = flag.Duration("accessDuration", 5*time.Minute, "The duration to allow access after a successful knock")
	allowedPeers       = []*allowedPeer{}
	knockSessions      = []*knockSession{}
	validKnockSequence = []int{}
)

type knockSession struct {
	ip     net.IP
	knocks []int
}

type allowedPeer struct {
	ip    net.IP
	start time.Time
	end   time.Time
}

func main() {
	flag.Parse()
	ctx := context.Background()
	log.Println("Starting port knock server...")
	parseKnockSequence()
	logKnockSequence()
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
	log.Println("Starting peer manager")
	go func() {
		log.Println("Started peer manager")
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
				if isPeerAllowed(peer) {
					log.Printf("Peer %s is already allowed\n", peer)
					logAllowedPeers()
					w.Write([]byte("You are already allowed access!"))
					return
				}
				has, s := peerHasKnockSession(peer)
				if !has {
					s := createKnockSessionForPeer(peer, port)
					log.Printf("Created knock session for peer %s: %#v", peer, s.knocks)
					w.Write([]byte("Knock, knock!"))
					return
				}
				s.knocks = append(s.knocks, port)
				if !knockSessionIsComplete(s) {
					log.Printf("Peer %s has an incomplete knock session: %#v\n", peer, s.knocks)
					w.Write([]byte("Knock, knock!"))
					return
				}
				log.Printf("Peer %s has a complete knock session: %#v\n", peer, s.knocks)
				allowPeer(peer)
				w.Write([]byte("Access granted!"))
			}))
		}(*basePort + i)
	}
}

func parseKnockSequence() {
	seq := strings.Split(*knockSequence, ",")
	validKnockSequence = make([]int, len(seq))
	for i, s := range seq {
		port, err := strconv.Atoi(s)
		if err != nil {
			log.Fatalf("Invalid port in knock sequence: %s\n", s)
		}
		validKnockSequence[i] = port
	}
}

func logKnockSequence() {
	log.Printf("Knock sequence: %v\n", validKnockSequence)
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

func peerHasKnockSession(peer net.IP) (bool, *knockSession) {
	for _, session := range knockSessions {
		if session.ip.Equal(peer) {
			return true, session
		}
	}
	return false, nil
}

func createKnockSessionForPeer(peer net.IP, port int) *knockSession {
	session := &knockSession{
		ip:     peer,
		knocks: []int{port},
	}
	knockSessions = append(knockSessions, session)
	return session
}

func knockSessionIsComplete(session *knockSession) bool {
	if len(session.knocks) != len(validKnockSequence) {
		return false
	}
	for i, port := range session.knocks {
		if port != validKnockSequence[i] {
			return false
		}
	}
	return true
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

func getHostPort(r *http.Request) string {
	_, hostPort, err := net.SplitHostPort(r.Host)
	if err != nil {
		return "-1"
	}
	return hostPort
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s :%s %s %s", r.Method, getHostPort(r), r.URL.Path, r.RemoteAddr)
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s :%s %s %s %s", r.Method, getHostPort(r), r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

func startHttpServer(ctx context.Context, port int, handler http.Handler) {
	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: loggingMiddleware(handler),
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
