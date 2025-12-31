package main

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
)

type rateLimiter interface {
	Wait(ctx context.Context) error
}

type grpcRateLimiter struct {
	mu    sync.Mutex
	rate  time.Duration
	burst int
	now   func() time.Time
	sleep func(context.Context, time.Duration) error

	tokens int
	last   time.Time
}

func newGrpcRateLimiter(rate time.Duration, burst int) *grpcRateLimiter {
	now := time.Now
	limiter := &grpcRateLimiter{
		rate:  rate,
		burst: burst,
		now:   now,
		sleep: sleepWithContext,
	}
	limiter.tokens = burst
	limiter.last = now()
	return limiter
}

func (r *grpcRateLimiter) Wait(ctx context.Context) error {
	if r == nil || r.rate <= 0 || r.burst <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.mu.Lock()
		now := r.now()
		r.refill(now)
		if r.tokens > 0 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}
		wait := r.rate - now.Sub(r.last)
		r.mu.Unlock()
		if wait <= 0 {
			continue
		}
		if err := r.sleep(ctx, wait); err != nil {
			return err
		}
	}
}

func (r *grpcRateLimiter) refill(now time.Time) {
	if r.rate <= 0 {
		r.tokens = r.burst
		r.last = now
		return
	}
	elapsed := now.Sub(r.last)
	if elapsed < r.rate {
		return
	}
	add := int(elapsed / r.rate)
	if add <= 0 {
		return
	}
	r.tokens += add
	if r.tokens > r.burst {
		r.tokens = r.burst
	}
	r.last = r.last.Add(time.Duration(add) * r.rate)
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

func rateLimitUnaryInterceptor(limiter rateLimiter) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				return nil, err
			}
		}
		return handler(ctx, req)
	}
}

func rateLimitStreamInterceptor(limiter rateLimiter) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if limiter == nil {
			return handler(srv, stream)
		}
		wrapped := &rateLimitedServerStream{
			ServerStream: stream,
			limiter:      limiter,
		}
		return handler(srv, wrapped)
	}
}
