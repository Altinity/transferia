package helpers

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/pkg/abstract/coordinator"
	"github.com/transferia/transferia/pkg/coordinator/s3coordinator"
)

const (
	CoordinatorBackendEnv  = "COORDINATOR_BACKEND"
	CoordinatorBackendFake = "fake"
	CoordinatorBackendS3   = "s3"
)

var (
	sharedS3CoordinatorOnce sync.Once
	sharedS3Coordinator     coordinator.Coordinator
	sharedS3CoordinatorErr  error
)

type coordinatorWithErrorCallbacks struct {
	coordinator.Coordinator
	onErrorCallback []func(err error)
}

func (c *coordinatorWithErrorCallbacks) FailReplication(transferID string, err error) error {
	for _, cb := range c.onErrorCallback {
		cb(err)
	}
	return c.Coordinator.FailReplication(transferID, err)
}

func CoordinatorBackend() string {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv(CoordinatorBackendEnv)))
	if backend == "" {
		return CoordinatorBackendFake
	}
	return backend
}

func NewCoordinatorForTransfer(t *testing.T, transferID string, onErrorCallback ...func(err error)) coordinator.Coordinator {
	t.Helper()
	if len(onErrorCallback) == 0 {
		onErrorCallback = append(onErrorCallback, func(err error) {
			require.NoError(t, err)
		})
	}

	switch CoordinatorBackend() {
	case CoordinatorBackendFake:
		return NewFakeCPErrRepl(onErrorCallback...)
	case CoordinatorBackendS3:
		cp, err := getSharedS3Coordinator()
		require.NoError(t, err)
		require.NoError(t, resetTransferState(cp, transferID))
		return &coordinatorWithErrorCallbacks{
			Coordinator:     cp,
			onErrorCallback: onErrorCallback,
		}
	default:
		require.FailNowf(t, "unsupported coordinator backend", "%s=%q", CoordinatorBackendEnv, CoordinatorBackend())
		return nil
	}
}

func getSharedS3Coordinator() (coordinator.Coordinator, error) {
	sharedS3CoordinatorOnce.Do(func() {
		sharedS3Coordinator, sharedS3CoordinatorErr = s3coordinator.NewS3Recipe(os.Getenv("S3_BUCKET"))
	})
	return sharedS3Coordinator, sharedS3CoordinatorErr
}

func resetTransferState(cp coordinator.Coordinator, transferID string) error {
	state, err := cp.GetTransferState(transferID)
	if err != nil {
		return err
	}
	if len(state) == 0 {
		return nil
	}

	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	return cp.RemoveTransferState(transferID, keys)
}
