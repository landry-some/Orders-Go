package main

import "time"

type grpcConfig struct {
	rateLimitInterval time.Duration
	rateLimitBurst    int
}

func loadGrpcConfigFromEnv() (grpcConfig, error) {
	cfg := grpcConfig{}
	var err error

	if cfg.rateLimitInterval, err = parseRequiredDuration("GRPC_RATE_LIMIT_INTERVAL"); err != nil {
		return cfg, err
	}
	if cfg.rateLimitBurst, err = parseRequiredInt("GRPC_RATE_LIMIT_BURST"); err != nil {
		return cfg, err
	}

	return cfg, nil
}
