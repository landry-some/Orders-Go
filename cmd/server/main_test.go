package main

import (
	"context"
	"testing"
)

func TestBuildLocationStoreRequiresRedisURL(t *testing.T) {
	t.Setenv("REDIS_URL", "")

	store, cleanup, err := buildLocationStore(context.Background())
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatalf("expected error when REDIS_URL is empty, got store=%v", store)
	}
}

func TestBuildLocationStoreRequiresDatabaseURL(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("DATABASE_URL", "")

	store, cleanup, err := buildLocationStore(context.Background())
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatalf("expected error when DATABASE_URL is empty, got store=%v", store)
	}
}
