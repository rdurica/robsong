// Package instance implements single-instance coordination over a unix socket.
package instance

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	sockName = "robsong.sock"
	// maxMessageBytes bounds what one connection may make the listener buffer.
	maxMessageBytes = 1 << 20
)

type message struct {
	Paths []string `json:"paths"`
}

// SocketPath returns the unix socket path used by the running instance.
func SocketPath() (string, error) {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, sockName), nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "robsong", sockName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "robsong", sockName), nil
}

// Send delivers paths to a running instance. It returns false, nil when no
// instance is listening.
func Send(paths []string) (bool, error) {
	path, err := SocketPath()
	if err != nil {
		return false, err
	}
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return false, nil
	}
	defer conn.Close()
	if paths == nil {
		paths = []string{}
	}
	b, err := json.Marshal(message{Paths: paths})
	if err != nil {
		return false, err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(append(b, '\n')); err != nil {
		return false, fmt.Errorf("send paths: %w", err)
	}
	return true, nil
}

type listener struct {
	l    net.Listener
	path string
	once sync.Once
}

// Listen starts accepting connections and calls onOpen (from a non-UI
// goroutine) with the paths of each one. A stale socket file is replaced.
func Listen(onOpen func(paths []string)) (io.Closer, error) {
	path, err := SocketPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create socket dir: %w", err)
	}
	if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
		_ = c.Close()
		return nil, errors.New("another instance is already listening")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale socket: %w", err)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	if ul, ok := l.(*net.UnixListener); ok {
		ul.SetUnlinkOnClose(false)
	}
	_ = os.Chmod(path, 0o600)
	ln := &listener{l: l, path: path}
	go ln.serve(onOpen)
	return ln, nil
}

func (ln *listener) serve(onOpen func([]string)) {
	for {
		conn, err := ln.l.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			line, err := bufio.NewReader(io.LimitReader(conn, maxMessageBytes)).ReadBytes('\n')
			if err != nil && len(line) == 0 {
				return
			}
			var m message
			if json.Unmarshal(line, &m) != nil {
				return
			}
			if m.Paths == nil {
				m.Paths = []string{}
			}
			onOpen(m.Paths)
		}()
	}
}

// Close stops listening and removes the socket file.
func (ln *listener) Close() error {
	var err error
	ln.once.Do(func() {
		err = ln.l.Close()
		if rerr := os.Remove(ln.path); rerr != nil && !os.IsNotExist(rerr) && err == nil {
			err = rerr
		}
	})
	return err
}
