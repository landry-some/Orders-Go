package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func parseOptionalDuration(name string) (*time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, nil
	}
	val, err := time.ParseDuration(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return nil, fmt.Errorf("%s must be >= 0", name)
	}
	return &val, nil
}

func parseOptionalInt(name string) (*int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, nil
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return nil, fmt.Errorf("%s must be >= 0", name)
	}
	return &val, nil
}

func parseRequiredInt(name string) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return 0, fmt.Errorf("%s must be >= 0", name)
	}
	return val, nil
}

func parseRequiredDuration(name string) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	val, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return 0, fmt.Errorf("%s must be >= 0", name)
	}
	return val, nil
}

func parseRequiredInt64(name string) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if val < 0 {
		return 0, fmt.Errorf("%s must be >= 0", name)
	}
	return val, nil
}

func parseOptionalBool(name string) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return false, nil
	}
	val, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return val, nil
}
