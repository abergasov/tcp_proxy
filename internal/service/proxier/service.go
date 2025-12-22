package proxier

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"tcp_proxy/internal/logger"
	"tcp_proxy/internal/notifier"
	"time"
)

type Config struct {
	ListenPort         int    `yaml:"listen_port"`
	DestinationPort    int    `yaml:"destination_port"`
	DestinationAddress string `yaml:"destination_address"`
	NotifyHTTP         bool   `yaml:"notify_http"`
}

type Service struct {
	ctx  context.Context
	log  logger.AppLogger
	conf *Config

	destinationAddr string
	notificator     notifier.Notificator
}

func NewService(ctx context.Context, conf *Config, log logger.AppLogger, notificator notifier.Notificator) *Service {
	destinationAddr := fmt.Sprintf("%s:%d", conf.DestinationAddress, conf.DestinationPort)
	return &Service{
		ctx:             ctx,
		conf:            conf,
		destinationAddr: destinationAddr,
		log: log.With(
			logger.WithService("proxier"),
			logger.WithString("destination_address", destinationAddr),
		),
		notificator: notificator,
	}
}

func (s *Service) Start() {
	s.log.Info("starting service")
	listener, err := net.Listen("tcp4", fmt.Sprintf(":%d", s.conf.ListenPort))
	if err != nil {
		s.log.Fatal("failed to start listener", err)
	}
	for {
		client, errA := listener.Accept()
		if errA != nil {
			s.log.Error("failed to accept client", errA)
			continue
		}
		l := s.log.With(logger.WithString("remote_ip", client.RemoteAddr().String()))
		l.Info("accepted new client")
		go s.handle(l, client)
	}
}

func (s *Service) handle(l logger.AppLogger, c net.Conn) {
	defer c.Close()

	br := bufio.NewReader(c)

	var prefix io.Reader // bytes we must send first (the parsed HTTP request)
	if s.conf.NotifyHTTP && looksLikeHTTP(br) {
		_ = c.SetReadDeadline(time.Now().Add(300 * time.Millisecond)) // avoid hanging on non-HTTP
		if req, err := http.ReadRequest(br); err == nil {
			body, _ := io.ReadAll(req.Body)
			_ = req.Body.Close()
			if err = s.notificator.SendInfoNewRequest(req, body, c.RemoteAddr().String(), s.destinationAddr); err != nil {
				payload := string(body)
				if len(payload) > 1024 {
					payload = payload[:1024] + "..."
				}
				l.Info("got http request",
					logger.WithString("key", req.Method),
					logger.WithString("payload", payload),
					logger.WithString("path", req.URL.String()),
				)
				l.Error("failed to send info request", err)
			}

			// rebuild raw request to forward
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(len(body))
			req.Header.Del("Transfer-Encoding") // avoid mismatch after ContentLength reset

			var buf bytes.Buffer
			if err = req.Write(&buf); err == nil {
				prefix = bytes.NewReader(buf.Bytes())
			}
		}
	}

	server, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", s.conf.DestinationAddress, s.conf.DestinationPort), 10*time.Second)
	if err != nil {
		l.Error("failed to connect to remote server", err)
		return
	}
	defer server.Close()

	src := io.Reader(br)
	if prefix != nil {
		src = io.MultiReader(prefix, br)
	}

	errCh := make(chan error, 2)
	go func() { _, e := io.Copy(server, src); errCh <- e }()
	go func() { _, e := io.Copy(c, server); errCh <- e }()
	<-errCh
}

func (s *Service) Stop() {
	s.log.Info("stopping service")
}

func looksLikeHTTP(br *bufio.Reader) bool {
	b, err := br.Peek(4)
	if err != nil {
		return false
	}

	knownPrefixes := [][]byte{
		[]byte("GET "),
		[]byte("POST"),
		[]byte("PUT "),
		[]byte("PATC"), // PATCH
		[]byte("DELE"), // DELETE
		[]byte("HEAD"),
		[]byte("OPTI"), // OPTIONS
		[]byte("CONN"), // CONNECT
		[]byte("PRI "), // h2c prior-knowledge preface
	}
	for _, prefix := range knownPrefixes {
		if bytes.HasPrefix(b, prefix) {
			return true
		}
	}
	return false
}
