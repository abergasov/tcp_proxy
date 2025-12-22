package test_utils

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type TestTCPServer struct {
	secretValidString   string
	secretInvalidString string
	commonCode          string

	ln   net.Listener
	addr string

	wg   sync.WaitGroup
	once sync.Once
}

func NewTestTCPServer(t *testing.T, commonCode string) *TestTCPServer {
	t.Helper()

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)

	s := &TestTCPServer{
		secretValidString:   uuid.NewString(),
		secretInvalidString: uuid.NewString(),
		commonCode:          commonCode,
		ln:                  ln,
		addr:                ln.Addr().String(),
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			c, errA := ln.Accept()
			if errA != nil {
				return
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer c.Close()

				_ = c.SetDeadline(time.Now().Add(3 * time.Second))
				r := bufio.NewReaderSize(c, 64*1024)

				// protocol: one line request: "PING <commonCode>\n"
				line, errR := r.ReadString('\n')
				if errR != nil {
					return
				}
				line = strings.TrimSpace(line)

				parts := strings.SplitN(line, " ", 2)
				if len(parts) != 2 || parts[0] != "PING" {
					_, _ = c.Write([]byte("ERR " + s.secretInvalidString + "\n"))
					return
				}
				if parts[1] != s.commonCode {
					_, _ = c.Write([]byte("ERR " + s.secretInvalidString + "\n"))
					return
				}
				_, _ = c.Write([]byte("OK " + s.secretValidString + "\n"))
			}()
		}
	}()

	t.Cleanup(s.Stop)
	return s
}

func (s *TestTCPServer) Stop() {
	s.once.Do(func() {
		_ = s.ln.Close()
		s.wg.Wait()
	})
}

func (s *TestTCPServer) GetDestinationPort() int {
	_, port, _ := net.SplitHostPort(s.addr)
	p, _ := strconv.Atoi(port)
	return p
}

func (s *TestTCPServer) GetSecretValidString() string   { return s.secretValidString }
func (s *TestTCPServer) GetSecretInvalidString() string { return s.secretInvalidString }
