package main

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type stubLimiter struct {
	calls int
	err   error
}

func (s *stubLimiter) Wait(ctx context.Context) error {
	s.calls++
	return s.err
}

type stubServerStream struct {
	ctx       context.Context
	recvCalls int
	recvErr   error
}

func (s *stubServerStream) Context() context.Context { return s.ctx }
func (s *stubServerStream) RecvMsg(m any) error {
	s.recvCalls++
	return s.recvErr
}
func (s *stubServerStream) SendMsg(m any) error { return nil }
func (s *stubServerStream) SetHeader(md metadata.MD) error {
	return nil
}
func (s *stubServerStream) SendHeader(md metadata.MD) error {
	return nil
}
func (s *stubServerStream) SetTrailer(md metadata.MD) {}

func TestRateLimitUnaryInterceptor_CallsLimiter(t *testing.T) {
	limiter := &stubLimiter{}
	interceptor := rateLimitUnaryInterceptor(limiter)

	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limiter.calls != 1 {
		t.Fatalf("expected limiter to be called once, got %d", limiter.calls)
	}
}

func TestRateLimitedServerStream_RecvMsgCallsLimiter(t *testing.T) {
	limiter := &stubLimiter{}
	stream := &stubServerStream{ctx: context.Background()}
	wrapped := &rateLimitedServerStream{
		ServerStream: stream,
		limiter:      limiter,
	}

	if err := wrapped.RecvMsg(&struct{}{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limiter.calls != 1 {
		t.Fatalf("expected limiter to be called once, got %d", limiter.calls)
	}
	if stream.recvCalls != 1 {
		t.Fatalf("expected recv to be called once, got %d", stream.recvCalls)
	}
}

func TestGrpcRateLimiter_Waits(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	var waits []time.Duration

	limiter := newGrpcRateLimiter(100*time.Millisecond, 1)
	limiter.now = func() time.Time { return now }
	limiter.last = now
	limiter.sleep = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		now = now.Add(d)
		return nil
	}

	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waits) != 1 || waits[0] != 100*time.Millisecond {
		t.Fatalf("expected one wait of 100ms, got %v", waits)
	}
}
