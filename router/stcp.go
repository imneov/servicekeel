package router

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"net"
	"sync"
)

// STCPConfig represents the configuration for STCP router
type STCPConfig struct {
	// SecretKey is used for authentication and encryption
	SecretKey string
	// AllowUsers specifies which users are allowed to connect
	AllowUsers []string
	// UseEncryption controls whether to encrypt the traffic
	UseEncryption bool
	// UseCompression controls whether to compress the traffic
	UseCompression bool
}

// STCPRouter implements the STCP protocol for secure TCP tunneling
type STCPRouter struct {
	config *STCPConfig
	ctx    context.Context
	cancel context.CancelFunc

	// server side
	serverListener net.Listener
	serverConns    sync.Map // map[string]net.Conn

	// visitor side
	visitorListener net.Listener
	visitorConns    sync.Map // map[string]net.Conn

	// auth cache
	authCache sync.Map // map[string]time.Time
}

// NewSTCPRouter creates a new STCP router instance
func NewSTCPRouter(config *STCPConfig) *STCPRouter {
	ctx, cancel := context.WithCancel(context.Background())
	return &STCPRouter{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
}

// StartServer starts the STCP server side
func (r *STCPRouter) StartServer(addr string) error {
	var err error
	r.serverListener, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go r.acceptServerConnections()
	return nil
}

// StartVisitor starts the STCP visitor side
func (r *STCPRouter) StartVisitor(addr string) error {
	var err error
	r.visitorListener, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go r.acceptVisitorConnections()
	return nil
}

// Close stops the router and closes all connections
func (r *STCPRouter) Close() {
	r.cancel()
	if r.serverListener != nil {
		r.serverListener.Close()
	}
	if r.visitorListener != nil {
		r.visitorListener.Close()
	}
}

// acceptServerConnections accepts incoming connections on the server side
func (r *STCPRouter) acceptServerConnections() {
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			conn, err := r.serverListener.Accept()
			if err != nil {
				continue
			}
			go r.handleServerConnection(conn)
		}
	}
}

// acceptVisitorConnections accepts incoming connections on the visitor side
func (r *STCPRouter) acceptVisitorConnections() {
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			conn, err := r.visitorListener.Accept()
			if err != nil {
				continue
			}
			go r.handleVisitorConnection(conn)
		}
	}
}

// handleServerConnection handles a new server connection
func (r *STCPRouter) handleServerConnection(conn net.Conn) {
	defer conn.Close()

	// Read authentication message
	authMsg := make([]byte, 32)
	if _, err := io.ReadFull(conn, authMsg); err != nil {
		return
	}

	// Verify authentication
	if !r.verifyAuth(authMsg) {
		return
	}

	// Generate a unique ID for this connection
	connID := generateConnID()
	r.serverConns.Store(connID, conn)
	defer r.serverConns.Delete(connID)

	// Handle the connection
	r.handleConnection(conn, connID, true)
}

// handleVisitorConnection handles a new visitor connection
func (r *STCPRouter) handleVisitorConnection(conn net.Conn) {
	defer conn.Close()

	// Read authentication message
	authMsg := make([]byte, 32)
	if _, err := io.ReadFull(conn, authMsg); err != nil {
		return
	}

	// Verify authentication
	if !r.verifyAuth(authMsg) {
		return
	}

	// Generate a unique ID for this connection
	connID := generateConnID()
	r.visitorConns.Store(connID, conn)
	defer r.visitorConns.Delete(connID)

	// Handle the connection
	r.handleConnection(conn, connID, false)
}

// handleConnection handles the data transfer between server and visitor
func (r *STCPRouter) handleConnection(conn net.Conn, connID string, isServer bool) {
	var remoteConn net.Conn

	// Find the corresponding remote connection
	if isServer {
		value, ok := r.visitorConns.Load(connID)
		if !ok {
			return
		}
		remoteConn = value.(net.Conn)
	} else {
		value, ok := r.serverConns.Load(connID)
		if !ok {
			return
		}
		remoteConn = value.(net.Conn)
	}

	// Create encrypted streams if needed
	var localStream, remoteStream io.ReadWriteCloser = conn, remoteConn
	if r.config.UseEncryption {
		var err error
		localStream, err = newEncryptedStream(conn, []byte(r.config.SecretKey))
		if err != nil {
			return
		}
		remoteStream, err = newEncryptedStream(remoteConn, []byte(r.config.SecretKey))
		if err != nil {
			return
		}
	}

	// Start bidirectional data transfer
	go func() {
		io.Copy(remoteStream, localStream)
		remoteStream.Close()
	}()
	io.Copy(localStream, remoteStream)
	localStream.Close()
}

// verifyAuth verifies the authentication message
func (r *STCPRouter) verifyAuth(authMsg []byte) bool {
	// TODO: Implement proper authentication
	return true
}

// generateConnID generates a unique connection ID
func generateConnID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return string(b)
}

// newEncryptedStream creates a new encrypted stream
func newEncryptedStream(conn net.Conn, key []byte) (io.ReadWriteCloser, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	return &encryptedStream{
		conn:  conn,
		block: block,
		iv:    iv,
	}, nil
}

// encryptedStream implements encrypted communication
type encryptedStream struct {
	conn  net.Conn
	block cipher.Block
	iv    []byte
}

func (s *encryptedStream) Read(p []byte) (n int, err error) {
	// Read IV
	if _, err := io.ReadFull(s.conn, s.iv); err != nil {
		return 0, err
	}

	// Read encrypted data
	ciphertext := make([]byte, len(p))
	n, err = s.conn.Read(ciphertext)
	if err != nil {
		return 0, err
	}

	// Decrypt
	stream := cipher.NewCFBDecrypter(s.block, s.iv)
	stream.XORKeyStream(p[:n], ciphertext[:n])
	return n, nil
}

func (s *encryptedStream) Write(p []byte) (n int, err error) {
	// Generate new IV
	if _, err := io.ReadFull(rand.Reader, s.iv); err != nil {
		return 0, err
	}

	// Write IV
	if _, err := s.conn.Write(s.iv); err != nil {
		return 0, err
	}

	// Encrypt and write
	ciphertext := make([]byte, len(p))
	stream := cipher.NewCFBEncrypter(s.block, s.iv)
	stream.XORKeyStream(ciphertext, p)
	return s.conn.Write(ciphertext)
}

func (s *encryptedStream) Close() error {
	return s.conn.Close()
}
