package grpcsrv

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// startMetricsServer starts a dedicated HTTP server for prometheus metrics.
func (s *Service) startMetricsServer(ctx context.Context) error {
	if s.metricsEndpoint == "" {
		return nil
	}

	metricsHandler := http.NewServeMux()
	metricsHandler.Handle("/metrics", promhttp.Handler())

	s.httpMetricsServer = &http.Server{
		Addr:              s.metricsEndpoint,
		Handler:           metricsHandler,
		ReadHeaderTimeout: s.httpReadHeaderTimeout,
	}

	listener, err := net.Listen("tcp", s.metricsEndpoint)
	if err != nil {
		return fmt.Errorf("%s. failed to start metrics server listener: %w", s.name, err)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		s.logger.Info(ctx, "starting metrics server", "addr", s.metricsEndpoint)
		if err := s.httpMetricsServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error(ctx, "metrics server error", "error", err)
		}
	}()

	return nil
}
