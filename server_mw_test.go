package grpcsrv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/n-r-w/ctxlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRegisterHealthCheckEndpoints(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockHealther := NewMockIHealther(ctrl)
	ctx := context.Background()

	// Create a test service with the mock health checker
	service := &Service{
		name:                 "test-service",
		logger:               ctxlog.NewStubWrapper(),
		healthCheckHandler:   mockHealther,
		livenessHandlerPath:  "/liveness",
		readinessHandlerPath: "/readiness",
	}

	// Create a new ServeMux
	mux := runtime.NewServeMux()

	// Register health check endpoints
	err := service.registerHealthCheckEndpoints(ctx, mux)
	require.NoError(t, err)

	// Apply the HEAD request middleware
	handler := service.setHeadRequestMiddleware(mux)

	// Test GET liveness endpoint
	mockHealther.EXPECT().LiveEndpoint(gomock.Any(), gomock.Any()).Times(1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/liveness", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test HEAD liveness endpoint
	mockHealther.EXPECT().LiveEndpoint(gomock.Any(), gomock.Any()).Times(1)
	req, err = http.NewRequestWithContext(ctx, http.MethodHead, "/liveness", nil)
	require.NoError(t, err)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test GET readiness endpoint
	mockHealther.EXPECT().ReadyEndpoint(gomock.Any(), gomock.Any()).Times(1)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, "/readiness", nil)
	require.NoError(t, err)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test HEAD readiness endpoint
	mockHealther.EXPECT().ReadyEndpoint(gomock.Any(), gomock.Any()).Times(1)
	req, err = http.NewRequestWithContext(ctx, http.MethodHead, "/readiness", nil)
	require.NoError(t, err)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
