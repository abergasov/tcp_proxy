package test_utils

import (
	"net"
	"path/filepath"
	"tcp_proxy/internal/config"
	"tcp_proxy/internal/utils"
	"testing"

	"github.com/stretchr/testify/require"
)

type RemoteServer interface {
	GetDestinationPort() int
}

func LoadTestConfig(t *testing.T) *config.AppConfig {
	t.Helper()
	path := filepath.Join(utils.LocateCurrentDirectory(), "configs", "app_conf.yml")
	cfg, err := config.LoadConfig(path)
	require.NoError(t, err, "failed to load config")
	return cfg
}

// GetFreePort generate a free tcp port for testing
func GetFreePort(t *testing.T) int {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	require.NoError(t, err)

	l, err := net.ListenTCP("tcp", addr)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, l.Close())
	}()
	return l.Addr().(*net.TCPAddr).Port
}
