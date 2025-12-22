package test_utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type TestPayload struct {
	SecretKey string
}

func NewTestPayload(secretKey string) *TestPayload {
	return &TestPayload{
		SecretKey: secretKey,
	}
}

func (tr *TestPayload) ToBytes() []byte {
	res, _ := json.Marshal(tr)
	return res
}

type TestHTTPServer struct {
	secretValidString   string
	secretInvalidString string
	commonCode          string

	srv *httptest.Server
}

func NewTestServer(t *testing.T, commonCode string) *TestHTTPServer {
	srv := &TestHTTPServer{
		secretValidString:   uuid.NewString(),
		secretInvalidString: uuid.NewString(),
		commonCode:          commonCode,
	}

	srv.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !slices.Contains([]string{http.MethodGet, http.MethodPost}, r.Method) || r.URL.Path != "/api/sample" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write(NewTestPayload(srv.secretInvalidString).ToBytes())
			return
		}
		if r.Method == http.MethodPost {
			// extract json from body
			var req TestPayload
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.Equal(t, req.SecretKey, srv.commonCode)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(NewTestPayload(srv.secretValidString).ToBytes())
	}))
	t.Cleanup(srv.srv.Close)
	return srv
}

func (s *TestHTTPServer) GetDestinationPort() int {
	data := strings.Split(s.srv.URL, ":")
	res, _ := strconv.Atoi(data[len(data)-1])
	return res
}

func (s *TestHTTPServer) GetSecretValidString() string {
	return s.secretValidString
}

func (s *TestHTTPServer) GetSecretInvalidString() string {
	return s.secretInvalidString
}
