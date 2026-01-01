package main

import (
	"context"
	"log"
	"strings"
	"time"

	"wayfinder/internal/observability"

	"google.golang.org/grpc"
)

type rateLimiter interface {
	Wait(ctx context.Context) error
}

type rateLimitedServerStream struct {
	grpc.ServerStream
	limiter rateLimiter
}

func (s *rateLimitedServerStream) RecvMsg(m any) error {
	if s.limiter != nil {
		if err := s.limiter.Wait(s.Context()); err != nil {
			return err
		}
	}
	return s.ServerStream.RecvMsg(m)
}

func rateLimitUnaryInterceptor(limiter rateLimiter, metrics *observability.Metrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		span := &observability.CallSpan{}
		start := time.Now()
		if metrics != nil && shouldTrackMethod(info.FullMethod) {
			span = metrics.Start(info.FullMethod)
		}
		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				span.End(err)
				return nil, err
			}
		}
		resp, err := handler(ctx, req)
		span.End(err)
		if err != nil && shouldTrackMethod(info.FullMethod) {
			log.Printf("grpc unary %s error after %v: %v", info.FullMethod, time.Since(start), err)
		}
		return resp, err
	}
}

func rateLimitStreamInterceptor(limiter rateLimiter, metrics *observability.Metrics) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		span := &observability.CallSpan{}
		start := time.Now()
		if metrics != nil && shouldTrackMethod(info.FullMethod) {
			span = metrics.Start(info.FullMethod)
		}
		if limiter == nil {
			err := handler(srv, stream)
			span.End(err)
			if err != nil && shouldTrackMethod(info.FullMethod) {
				log.Printf("grpc stream %s error after %v: %v", info.FullMethod, time.Since(start), err)
			}
			return err
		}
		wrapped := &rateLimitedServerStream{
			ServerStream: stream,
			limiter:      limiter,
		}
		err := handler(srv, wrapped)
		span.End(err)
		if err != nil && shouldTrackMethod(info.FullMethod) {
			log.Printf("grpc stream %s error after %v: %v", info.FullMethod, time.Since(start), err)
		}
		return err
	}
}

func shouldTrackMethod(method string) bool {
	return method != "" && !strings.HasPrefix(method, "/grpc.reflection.")
}
