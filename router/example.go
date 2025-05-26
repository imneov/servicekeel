package router

import (
	"fmt"
	"log"
	"net"
	"time"
)

func Example() {
	// Create router instance
	config := &STCPConfig{
		SecretKey:      "your-secret-key",
		UseEncryption:  true,
		UseCompression: false,
	}
	router := NewSTCPRouter(config)

	// Start server and visitor
	err := router.StartServer("127.0.0.1:7000")
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	err = router.StartVisitor("127.0.0.1:7001")
	if err != nil {
		log.Fatalf("Failed to start visitor: %v", err)
	}

	// Create a simple echo server
	echoServer, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		log.Fatalf("Failed to create echo server: %v", err)
	}
	defer echoServer.Close()

	// Start echo server
	go func() {
		for {
			conn, err := echoServer.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				buf := make([]byte, 1024)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					conn.Write(buf[:n])
				}
			}(conn)
		}
	}()

	// Connect to STCP server
	serverConn, err := net.Dial("tcp", "127.0.0.1:7000")
	if err != nil {
		log.Fatalf("Failed to connect to STCP server: %v", err)
	}
	defer serverConn.Close()

	// Connect to STCP visitor
	visitorConn, err := net.Dial("tcp", "127.0.0.1:7001")
	if err != nil {
		log.Fatalf("Failed to connect to STCP visitor: %v", err)
	}
	defer visitorConn.Close()

	// Test data transfer
	testData := []byte("Hello, STCP!")
	_, err = serverConn.Write(testData)
	if err != nil {
		log.Fatalf("Failed to write test data: %v", err)
	}

	buf := make([]byte, len(testData))
	_, err = visitorConn.Read(buf)
	if err != nil {
		log.Fatalf("Failed to read test data: %v", err)
	}

	fmt.Printf("Received: %s\n", string(buf))

	// Cleanup
	router.Close()
}

func ExampleConcurrent() {
	// Create router instance
	config := &STCPConfig{
		SecretKey:      "your-secret-key",
		UseEncryption:  true,
		UseCompression: false,
	}
	router := NewSTCPRouter(config)

	// Start server and visitor
	err := router.StartServer("127.0.0.1:7000")
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	err = router.StartVisitor("127.0.0.1:7001")
	if err != nil {
		log.Fatalf("Failed to start visitor: %v", err)
	}

	// Create a simple echo server
	echoServer, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		log.Fatalf("Failed to create echo server: %v", err)
	}
	defer echoServer.Close()

	// Start echo server
	go func() {
		for {
			conn, err := echoServer.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				buf := make([]byte, 1024)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					conn.Write(buf[:n])
				}
			}(conn)
		}
	}()

	// Test concurrent connections
	const numConnections = 10
	done := make(chan bool)

	for i := 0; i < numConnections; i++ {
		go func(id int) {
			// Connect to STCP server
			serverConn, err := net.Dial("tcp", "127.0.0.1:7000")
			if err != nil {
				log.Printf("Failed to connect to STCP server: %v", err)
				done <- true
				return
			}
			defer serverConn.Close()

			// Connect to STCP visitor
			visitorConn, err := net.Dial("tcp", "127.0.0.1:7001")
			if err != nil {
				log.Printf("Failed to connect to STCP visitor: %v", err)
				done <- true
				return
			}
			defer visitorConn.Close()

			// Test data transfer
			testData := []byte(fmt.Sprintf("Hello, STCP! Connection %d", id))
			_, err = serverConn.Write(testData)
			if err != nil {
				log.Printf("Failed to write test data: %v", err)
				done <- true
				return
			}

			buf := make([]byte, len(testData))
			_, err = visitorConn.Read(buf)
			if err != nil {
				log.Printf("Failed to read test data: %v", err)
				done <- true
				return
			}

			fmt.Printf("Connection %d received: %s\n", id, string(buf))
			done <- true
		}(i)
	}

	// Wait for all connections to complete
	for i := 0; i < numConnections; i++ {
		select {
		case <-done:
			continue
		case <-time.After(5 * time.Second):
			log.Fatal("Test timed out")
		}
	}

	// Cleanup
	router.Close()
}
