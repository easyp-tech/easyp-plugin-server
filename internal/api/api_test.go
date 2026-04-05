package api

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/easyp-tech/service/internal/core"
)

func TestErrorToStatus_FeatureDenied(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "direct", err: core.ErrFeatureDenied},
		{name: "wrapped", err: fmt.Errorf("wrapper: %w", core.ErrFeatureDenied)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := ErrorToStatus(tt.err)
			if s.Code() != codes.PermissionDenied {
				t.Errorf("ErrorToStatus(%v).Code() = %v, want PermissionDenied", tt.err, s.Code())
			}
		})
	}
}

func TestErrorToStatus_ExistingErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		want codes.Code
	}{
		{core.ErrNotFound, codes.NotFound},
		{core.ErrAlreadyExists, codes.AlreadyExists},
		{core.ErrInvalidPluginName, codes.InvalidArgument},
		{core.ErrMaxPluginsExceeded, codes.ResourceExhausted},
		{core.ErrServerOverloaded, codes.ResourceExhausted},
		{errors.New("unknown"), codes.Internal},
		{nil, codes.OK},
	}

	for _, tt := range tests {
		s := ErrorToStatus(tt.err)
		if s.Code() != tt.want {
			t.Errorf("ErrorToStatus(%v).Code() = %v, want %v", tt.err, s.Code(), tt.want)
		}
	}
}
