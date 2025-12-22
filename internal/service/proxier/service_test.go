package proxier_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"tcp_proxy/internal/service/proxier"
	"tcp_proxy/internal/test_utils"
	"tcp_proxy/internal/utils"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestServiceHTTPRequest(t *testing.T) {
	// given
	container := test_utils.GetClean(t)
	commonCode := uuid.NewString()
	proxyPort := test_utils.GetFreePort(t)
	testSrv := test_utils.NewTestServer(t, commonCode)

	srvProxy := proxier.NewService(container.Ctx, generateConfig(t, testSrv, proxyPort), container.Log, container.SrvNotificatorMock)
	t.Cleanup(srvProxy.Stop)
	go srvProxy.Start()

	validURL := fmt.Sprintf("http://127.0.0.1:%d/api/sample", proxyPort)
	invalidURL := fmt.Sprintf("http://127.0.0.1:%d/abc", proxyPort)
	container.SrvNotificatorMock.EXPECT().SendInfoNewRequest(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(4)

	// wait until proxy accepts
	require.Eventually(t, func() bool {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 100*time.Millisecond)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 5*time.Second, 100*time.Millisecond)
	t.Run("valid get http_request", func(t *testing.T) {
		// when
		resp, code, err := utils.GetCurl[test_utils.TestPayload](container.Ctx, validURL, nil)
		require.NoError(t, err)

		// then
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, resp.SecretKey, testSrv.GetSecretValidString())
		time.Sleep(5 * time.Second) // utils.GetCurl reuses same http client
	})
	t.Run("valid post http_request", func(t *testing.T) {
		// when
		resp, code, err := utils.PostCurl[test_utils.TestPayload](container.Ctx, validURL, test_utils.NewTestPayload(commonCode), nil)
		require.NoError(t, err)

		// then
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, resp.SecretKey, testSrv.GetSecretValidString())
		time.Sleep(5 * time.Second)
	})
	t.Run("invalid http_request", func(t *testing.T) {
		// when
		resp, code, err := utils.PostCurl[test_utils.TestPayload](container.Ctx, invalidURL, test_utils.NewTestPayload(commonCode), nil)
		require.NoError(t, err)

		// then
		require.Equal(t, http.StatusMethodNotAllowed, code)
		require.Equal(t, resp.SecretKey, testSrv.GetSecretInvalidString())
		time.Sleep(5 * time.Second)
	})
	t.Run("invalid patch http_request", func(t *testing.T) {
		// when
		resp, code, err := utils.PatchCurl[test_utils.TestPayload](container.Ctx, invalidURL, test_utils.NewTestPayload(commonCode), nil)
		require.NoError(t, err)

		// then
		require.Equal(t, http.StatusMethodNotAllowed, code)
		require.Equal(t, resp.SecretKey, testSrv.GetSecretInvalidString())
		time.Sleep(5 * time.Second)
	})
}

func TestServiceTCPRequest(t *testing.T) {
	container := test_utils.GetClean(t)
	commonCode := uuid.NewString()
	proxyPort := test_utils.GetFreePort(t)
	testTCPSrv := test_utils.NewTestTCPServer(t, commonCode)

	srvProxy := proxier.NewService(container.Ctx, generateConfig(t, testTCPSrv, proxyPort), container.Log, container.SrvNotificatorMock)
	t.Cleanup(srvProxy.Stop)
	go srvProxy.Start()

	addr := fmt.Sprintf("127.0.0.1:%d", proxyPort)

	// wait until proxy accepts
	require.Eventually(t, func() bool {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 100*time.Millisecond)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 5*time.Second, 100*time.Millisecond)
	t.Run("valid tcp request", func(t *testing.T) {
		resp := tcpRoundTrip(t, container.Ctx, addr, "PING "+commonCode+"\n")
		require.Equal(t, "OK "+testTCPSrv.GetSecretValidString(), resp)
	})

	t.Run("invalid tcp request: wrong cmd", func(t *testing.T) {
		resp := tcpRoundTrip(t, container.Ctx, addr, "HELLO "+commonCode+"\n")
		require.Equal(t, "ERR "+testTCPSrv.GetSecretInvalidString(), resp)
	})

	t.Run("invalid tcp request: wrong secret", func(t *testing.T) {
		resp := tcpRoundTrip(t, container.Ctx, addr, "PING "+uuid.NewString()+"\n")
		require.Equal(t, "ERR "+testTCPSrv.GetSecretInvalidString(), resp)
	})
}

func TestServiceGRPCRequest(t *testing.T) {
	// given
	container := test_utils.GetClean(t)
	container.SrvNotificatorMock.EXPECT().SendInfoNewRequest(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	proxyPort := test_utils.GetFreePort(t)

	grpcSrv := test_utils.NewTestGRPCServer(t, test_utils.SelfSignedCert(t))

	srvProxy := proxier.NewService(container.Ctx, generateConfig(t, grpcSrv, proxyPort), container.Log, container.SrvNotificatorMock)
	t.Cleanup(srvProxy.Stop)
	go srvProxy.Start()

	// wait until proxy accepts
	require.Eventually(t, func() bool {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 100*time.Millisecond)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 2*time.Second, 20*time.Millisecond)

	// when
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", proxyPort),
		grpc.WithTransportCredentials(credentials.NewTLS(grpcSrv.ClientTLSConfig())),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// then
	payload := generateRandomPayload(t, 32*1024) // 32KB payload
	var out wrapperspb.BytesValue
	in := &wrapperspb.BytesValue{Value: payload}
	err = conn.Invoke(container.Ctx, test_utils.EchoMethod, in, &out)
	require.NoError(t, err)

	require.True(t, bytes.HasPrefix(out.Value, []byte("pong:")))
	require.True(t, bytes.Equal(out.Value[len("pong:"):], in.Value))
}

func tcpRoundTrip(t *testing.T, ctx context.Context, addr, msg string) string {
	t.Helper()

	d := net.Dialer{Timeout: 2 * time.Second}
	c, err := d.DialContext(ctx, "tcp", addr)
	require.NoError(t, err)
	defer c.Close()

	_ = c.SetDeadline(time.Now().Add(2 * time.Second))

	_, err = c.Write([]byte(msg))
	require.NoError(t, err)

	br := bufio.NewReader(c)
	line, err := br.ReadString('\n')
	require.NoError(t, err)

	return trimNL(line)
}

func trimNL(s string) string {
	for len(s) > 0 {
		last := s[len(s)-1]
		if last == '\n' || last == '\r' {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return s
}

func generateConfig(t *testing.T, srv test_utils.RemoteServer, proxyPort int) *proxier.Config {
	t.Helper()
	return &proxier.Config{
		ListenPort:         proxyPort,
		DestinationAddress: "127.0.0.1",
		DestinationPort:    srv.GetDestinationPort(),
		NotifyHTTP:         true,
	}
}

func generateRandomPayload(t *testing.T, size int) []byte {
	payload := make([]byte, size)
	_, err := rand.Read(payload)
	require.NoError(t, err)
	return payload
}
