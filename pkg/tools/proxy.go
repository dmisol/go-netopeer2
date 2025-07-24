// proxy_server.go
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	proxySocketPath  = "/tmp/proxy.sock"
	targetSocketPath = "/tmp/netopeer2.sock"
)

type ProxyServer struct {
	listener     net.Listener
	targetSocket string
	connections  map[string]*ProxyConnection
	connMutex    sync.RWMutex
	logger       *log.Logger
}

type ProxyConnection struct {
	id              string
	clientConn      net.Conn
	targetConn      net.Conn
	bytesFromClient int64
	bytesToClient   int64
	startTime       time.Time
}

func NewProxyServer(listenSocket, targetSocket string) *ProxyServer {
	return &ProxyServer{
		targetSocket: targetSocket,
		connections:  make(map[string]*ProxyConnection),
		logger:       log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (ps *ProxyServer) Start() error {
	// Remove existing socket
	if err := os.RemoveAll(proxySocketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %v", err)
	}

	// Create listener
	listener, err := net.Listen("unix", proxySocketPath)
	if err != nil {
		return fmt.Errorf("failed to create listener: %v", err)
	}
	ps.listener = listener

	// Set permissions
	if err := os.Chmod(proxySocketPath, 0666); err != nil {
		ps.logger.Printf("Warning: failed to set socket permissions: %v", err)
	}

	ps.logger.Printf("Proxy server listening on %s", proxySocketPath)
	ps.logger.Printf("Forwarding to target socket: %s", ps.targetSocket)

	// Setup graceful shutdown
	ps.setupGracefulShutdown()

	// Accept connections
	for {
		clientConn, err := ps.listener.Accept()
		if err != nil {
			ps.logger.Printf("Accept error: %v", err)
			continue
		}

		go ps.handleConnection(clientConn)
	}
}

func (ps *ProxyServer) handleConnection(clientConn net.Conn) {
	connID := fmt.Sprintf("conn-%d", time.Now().UnixNano())
	ps.logger.Printf("[%s] New client connection from %v", connID, clientConn.RemoteAddr())

	// Connect to target server
	targetConn, err := net.Dial("unix", ps.targetSocket)
	if err != nil {
		ps.logger.Printf("[%s] Failed to connect to target: %v", connID, err)
		clientConn.Close()
		return
	}

	// Create proxy connection
	proxyConn := &ProxyConnection{
		id:         connID,
		clientConn: clientConn,
		targetConn: targetConn,
		startTime:  time.Now(),
	}

	// Register connection
	ps.connMutex.Lock()
	ps.connections[connID] = proxyConn
	ps.connMutex.Unlock()

	// Start proxying data
	ps.proxyData(proxyConn)

	// Cleanup
	ps.connMutex.Lock()
	delete(ps.connections, connID)
	ps.connMutex.Unlock()

	duration := time.Since(proxyConn.startTime)
	ps.logger.Printf("[%s] Connection closed. Duration: %v, Client->Target: %d bytes, Target->Client: %d bytes",
		connID, duration, proxyConn.bytesFromClient, proxyConn.bytesToClient)
}

func (ps *ProxyServer) proxyData(proxyConn *ProxyConnection) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Client to Target
	go func() {
		defer wg.Done()
		defer proxyConn.targetConn.Close()

		written, err := ps.copyWithLogging(proxyConn.targetConn, proxyConn.clientConn,
			proxyConn.id, "Cl->Srv", &proxyConn.bytesFromClient)

		if err != nil && err != io.EOF {
			ps.logger.Printf("[%s] Client->Target copy error: %v", proxyConn.id, err)
		}
		ps.logger.Printf("[%s] Client->Target finished, %d bytes transferred", proxyConn.id, written)
	}()

	// Target to Client
	go func() {
		defer wg.Done()
		defer proxyConn.clientConn.Close()

		written, err := ps.copyWithLogging(proxyConn.clientConn, proxyConn.targetConn,
			proxyConn.id, "Srv->Cl", &proxyConn.bytesToClient)

		if err != nil && err != io.EOF {
			ps.logger.Printf("[%s] Target->Client copy error: %v", proxyConn.id, err)
		}
		ps.logger.Printf("[%s] Target->Client finished, %d bytes transferred", proxyConn.id, written)
	}()

	wg.Wait()
}

func (ps *ProxyServer) copyWithLogging(dst io.Writer, src io.Reader, connID, direction string, counter *int64) (int64, error) {
	buffer := make([]byte, 32*1024) // 32KB buffer
	var totalBytes int64

	for {
		nr, err := src.Read(buffer)
		if nr > 0 {
			// Log the data being transferred (optional - can be verbose)
			// ps.logger.Printf("[%s] %s: %d bytes", connID, direction, nr)

			// Optionally log the actual data (be careful with binary data)
			//if nr < 1024 { // Only log small messages to avoid spam
			ps.logger.Printf("[%s] %s\n%s\n", connID, direction, string(buffer[:nr])) //html.UnescapeString(string(buffer[:nr])))
			//}

			nw, err := dst.Write(buffer[:nr])
			if nw > 0 {
				totalBytes += int64(nw)
				*counter += int64(nw)
			}
			if err != nil {
				return totalBytes, err
			}
			if nr != nw {
				return totalBytes, io.ErrShortWrite
			}
		}
		if err != nil {
			return totalBytes, err
		}
	}
}

func (ps *ProxyServer) setupGracefulShutdown() {
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		ps.logger.Println("Shutting down proxy server...")

		// Close all active connections
		ps.connMutex.RLock()
		for _, conn := range ps.connections {
			conn.clientConn.Close()
			conn.targetConn.Close()
		}
		ps.connMutex.RUnlock()

		// Close listener
		if ps.listener != nil {
			ps.listener.Close()
		}

		// Remove socket file
		os.RemoveAll(proxySocketPath)

		ps.logger.Println("Proxy server shutdown complete")
		os.Exit(0)
	}()
}

func (ps *ProxyServer) GetStats() {
	ps.connMutex.RLock()
	defer ps.connMutex.RUnlock()

	ps.logger.Printf("Active connections: %d", len(ps.connections))
	for id, conn := range ps.connections {
		duration := time.Since(conn.startTime)
		ps.logger.Printf("  [%s] Duration: %v, From Client: %d bytes, To Client: %d bytes",
			id, duration, conn.bytesFromClient, conn.bytesToClient)
	}
}

func main() {
	proxy := NewProxyServer(proxySocketPath, targetSocketPath)

	// Start stats reporting goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			proxy.GetStats()
		}
	}()

	if err := proxy.Start(); err != nil {
		log.Fatal("Failed to start proxy server:", err)
	}
}
