package main

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"wayfinder/internal/observability"

	"google.golang.org/grpc"
)

type rateLimiter interface {
	Wait(ctx context.Context) error
}

// grpcRateLimiter is a simple token bucket limiter.
type grpcRateLimiter struct {
	mu     sync.Mutex
	rate   time.Duration
	burst  int
	now    func() time.Time
	sleep  func(context.Context, time.Duration) error
	onWait func(time.Duration)

	tokens int
	last   time.Time
}

func newGrpcRateLimiter(rate time.Duration, burst int, onWait func(time.Duration)) *grpcRateLimiter {
	now := time.Now
	limiter := &grpcRateLimiter{
		rate:   rate,
		burst:  burst,
		now:    now,
		sleep:  sleepWithContext,
		onWait: onWait,
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
		if r.onWait != nil {
			r.onWait(wait)
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
