package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// MultiListener 实现了 net.Listener 接口，同时监听 TCP 和 Unix socket
type MultiListener struct {
	tcpListener  net.Listener
	unixListener net.Listener
	connChan     chan net.Conn
	errChan      chan error
	closeChan    chan struct{}
	once         sync.Once
}

// NewMultiListener 创建一个新的 MultiListener
func NewMultiListener(tcpAddr, unixAddr string) (*MultiListener, error) {
	// 如果 Unix socket 文件已存在，先删除
	if err := os.RemoveAll(unixAddr); err != nil {
		return nil, fmt.Errorf("remove socket file error: %v", err)
	}

	// 创建 TCP 监听器
	tcpListener, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		return nil, fmt.Errorf("TCP listen error: %v", err)
	}

	// 创建 Unix socket 监听器
	unixListener, err := net.Listen("unix", unixAddr)
	if err != nil {
		tcpListener.Close()
		return nil, fmt.Errorf("Unix socket listen error: %v", err)
	}

	ml := &MultiListener{
		tcpListener:  tcpListener,
		unixListener: unixListener,
		connChan:     make(chan net.Conn),
		errChan:      make(chan error, 2),
		closeChan:    make(chan struct{}),
	}

	// 启动监听协程
	go ml.acceptLoop(tcpListener, "TCP")
	go ml.acceptLoop(unixListener, "Unix")

	return ml, nil
}

// acceptLoop 处理连接接受循环
func (ml *MultiListener) acceptLoop(listener net.Listener, connType string) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ml.closeChan:
				return
			default:
				ml.errChan <- fmt.Errorf("%s accept error: %v", connType, err)
				return
			}
		}
		select {
		case ml.connChan <- conn:
		case <-ml.closeChan:
			conn.Close()
			return
		}
	}
}

// Accept 实现 net.Listener 接口
func (ml *MultiListener) Accept() (net.Conn, error) {
	select {
	case conn := <-ml.connChan:
		return conn, nil
	case err := <-ml.errChan:
		return nil, err
	case <-ml.closeChan:
		return nil, fmt.Errorf("listener closed")
	}
}

// Close 实现 net.Listener 接口
func (ml *MultiListener) Close() error {
	var err error
	ml.once.Do(func() {
		close(ml.closeChan)
		if err1 := ml.tcpListener.Close(); err1 != nil {
			err = err1
		}
		if err2 := ml.unixListener.Close(); err2 != nil {
			err = err2
		}
	})
	return err
}

// Addr 实现 net.Listener 接口
func (ml *MultiListener) Addr() net.Addr {
	return ml.tcpListener.Addr()
}

func main() {
	// 创建一个通道用于接收系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建 MultiListener
	listener, err := NewMultiListener(":8080", "/tmp/example.sock")
	if err != nil {
		log.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	log.Printf("Server listening on TCP :8080 and Unix socket /tmp/example.sock")

	// 处理连接
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			return
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("New connection from %s", conn.RemoteAddr().String())

	// 这里处理连接
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("Read error: %v", err)
			return
		}
		log.Printf("Received %d bytes: %s", n, string(buf[:n]))
	}
}
