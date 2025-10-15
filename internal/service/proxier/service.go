package proxier

import (
	"context"
	"fmt"
	"io"
	"net"
	"tcp_proxy/internal/config"
	"tcp_proxy/internal/logger"
)

type Service struct {
	ctx  context.Context
	log  logger.AppLogger
	conf *config.AppConfig
}

func NewService(ctx context.Context, conf *config.AppConfig, log logger.AppLogger) *Service {
	return &Service{
		ctx:  ctx,
		conf: conf,
		log:  log.With(logger.WithService("proxier")),
	}
}

func (s *Service) Start() {
	s.log.Info("starting service")
	listener, err := net.Listen("tcp4", fmt.Sprintf(":%d", s.conf.AppPort))
	if err != nil {
		s.log.Fatal("failed to start listener", err)
	}
	for {
		client, errA := listener.Accept()
		if errA != nil {
			s.log.Error("failed to accept client", errA)
			continue
		}
		s.log.Info("accepted new client")
		go s.handle(client)
	}
}

func (s *Service) handle(c net.Conn) {
	defer c.Close()
	server, err := net.Dial("tcp", fmt.Sprintf("%s:%d", s.conf.DestinationAddress, s.conf.DestinationPort))
	if err != nil {
		s.log.Fatal("failed to connect to remote server", err)
	}
	defer server.Close()

	go io.Copy(server, c)
	io.Copy(c, server)
}

func (s *Service) Stop() {
	s.log.Info("stopping service")
}
