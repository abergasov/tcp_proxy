package test_utils

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type EchoServiceServer interface{} // must be interface

type TestGRPCServer struct {
	ln      net.Listener
	srv     *grpc.Server
	addr    string
	once    sync.Once
	tlsConf *tls.Config
}

const EchoMethod = "/test.EchoService/EchoBytes"

func NewTestGRPCServer(t *testing.T, cert tls.Certificate, secureConnection bool) *TestGRPCServer {
	t.Helper()

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)

	s := grpc.NewServer()
	if secureConnection {
		s = grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
		})))
	}

	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "test.EchoService",
		HandlerType: (*EchoServiceServer)(nil), // <- FIX
		Methods: []grpc.MethodDesc{
			{
				MethodName: "EchoBytes",
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := new(wrapperspb.BytesValue)
					if err := dec(in); err != nil {
						return nil, err
					}

					h := func(ctx context.Context, req any) (any, error) {
						b := req.(*wrapperspb.BytesValue).Value
						out := &wrapperspb.BytesValue{Value: append([]byte("pong:"), b...)}
						return out, nil
					}

					if interceptor == nil {
						return h(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: EchoMethod,
					}
					return interceptor(ctx, in, info, h)
				},
			},
		},
	}, struct{}{}) // implements empty interface

	ts := &TestGRPCServer{
		ln:      ln,
		srv:     s,
		addr:    ln.Addr().String(),
		tlsConf: &tls.Config{InsecureSkipVerify: true},
	}

	go func() { _ = s.Serve(ln) }()
	t.Cleanup(ts.Stop)
	return ts
}

func (s *TestGRPCServer) Stop() {
	s.once.Do(func() {
		s.srv.GracefulStop()
		_ = s.ln.Close()
	})
}

func (s *TestGRPCServer) GetDestinationPort() int {
	_, port, _ := net.SplitHostPort(s.addr)
	p, _ := strconv.Atoi(port)
	return p
}

func (s *TestGRPCServer) ClientTLSConfig() *tls.Config { return s.tlsConf }

func SelfSignedCert(t *testing.T) tls.Certificate {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)

	tpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return cert
}
