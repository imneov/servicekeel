package router

import (
	"net"
	"testing"
	"time"
)

func TestSTCPRouter(t *testing.T) {
	// Create router instance
	config := &STCPConfig{
		SecretKey:      "test-secret-key",
		UseEncryption:  true,
		UseCompression: false,
	}
	router := NewSTCPRouter(config)

	// Start server and visitor
	err := router.StartServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	err = router.StartVisitor("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start visitor: %v", err)
	}

	// Get server and visitor addresses
	serverAddr := router.serverListener.Addr().String()
	visitorAddr := router.visitorListener.Addr().String()

	// Create test server
	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer server.Close()

	// Start test server
	go func() {
		conn, err := server.Accept()
		if err != nil {
			t.Errorf("Failed to accept connection: %v", err)
			return
		}
		defer conn.Close()

		// Echo back any data received
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			conn.Write(buf[:n])
		}
	}()

	// Connect to server through STCP
	serverConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Failed to connect to STCP server: %v", err)
	}
	defer serverConn.Close()

	// Connect to visitor through STCP
	visitorConn, err := net.Dial("tcp", visitorAddr)
	if err != nil {
		t.Fatalf("Failed to connect to STCP visitor: %v", err)
	}
	defer visitorConn.Close()

	// Test data transfer
	testData := []byte("Hello, STCP!")
	_, err = serverConn.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	buf := make([]byte, len(testData))
	_, err = visitorConn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	if string(buf) != string(testData) {
		t.Errorf("Data mismatch: got %s, want %s", string(buf), string(testData))
	}

	// Test bidirectional communication
	testData = []byte("Hello from visitor!")
	_, err = visitorConn.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write test data from visitor: %v", err)
	}

	buf = make([]byte, len(testData))
	_, err = serverConn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read test data on server: %v", err)
	}

	if string(buf) != string(testData) {
		t.Errorf("Data mismatch: got %s, want %s", string(buf), string(testData))
	}

	// Cleanup
	router.Close()
}

func TestSTCPRouterConcurrent(t *testing.T) {
	// Create router instance
	config := &STCPConfig{
		SecretKey:      "test-secret-key",
		UseEncryption:  true,
		UseCompression: false,
	}
	router := NewSTCPRouter(config)

	// Start server and visitor
	err := router.StartServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	err = router.StartVisitor("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start visitor: %v", err)
	}

	// Get server and visitor addresses
	serverAddr := router.serverListener.Addr().String()
	visitorAddr := router.visitorListener.Addr().String()

	// Create test server
	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer server.Close()

	// Start test server
	go func() {
		for {
			conn, err := server.Accept()
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
			// Connect to server through STCP
			serverConn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				t.Errorf("Failed to connect to STCP server: %v", err)
				done <- true
				return
			}
			defer serverConn.Close()

			// Connect to visitor through STCP
			visitorConn, err := net.Dial("tcp", visitorAddr)
			if err != nil {
				t.Errorf("Failed to connect to STCP visitor: %v", err)
				done <- true
				return
			}
			defer visitorConn.Close()

			// Test data transfer
			testData := []byte("Hello, STCP!")
			_, err = serverConn.Write(testData)
			if err != nil {
				t.Errorf("Failed to write test data: %v", err)
				done <- true
				return
			}

			buf := make([]byte, len(testData))
			_, err = visitorConn.Read(buf)
			if err != nil {
				t.Errorf("Failed to read test data: %v", err)
				done <- true
				return
			}

			if string(buf) != string(testData) {
				t.Errorf("Data mismatch: got %s, want %s", string(buf), string(testData))
				done <- true
				return
			}

			done <- true
		}(i)
	}

	// Wait for all connections to complete
	for i := 0; i < numConnections; i++ {
		select {
		case <-done:
			continue
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out")
		}
	}

	// Cleanup
	router.Close()
}
