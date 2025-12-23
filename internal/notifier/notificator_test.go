package notifier_test

import (
	"tcp_proxy/internal/test_utils"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlackNotificator_SendErrMessage(t *testing.T) {
	container := test_utils.GetClean(t)
	require.NoError(t, container.SrvNotificator.SendInfoMessage("test message", "a", "b", "c"))
}
