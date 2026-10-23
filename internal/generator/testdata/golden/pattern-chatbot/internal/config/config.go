package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address         string
	LogLevel        string
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
	MaxBodyBytes    int64
}

type Loader interface{ Load() (Config, error) }
type EnvLoader struct{ Getenv func(string) string }

func (e EnvLoader) Load() (Config, error) {
	c := Config{Address: ":8080", LogLevel: "info", RequestTimeout: 10 * time.Second, ShutdownTimeout: 10 * time.Second, MaxBodyBytes: 1 << 20}
	if v := e.Getenv("HTTP_ADDR"); v != "" {
		c.Address = v
	}
	_, port, err := net.SplitHostPort(c.Address)
	if err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR must be host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return Config{}, fmt.Errorf("HTTP_ADDR port must be between 0 and 65535")
	}
	if v := e.Getenv("LOG_LEVEL"); v != "" {
		c.LogLevel = strings.ToLower(v)
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("invalid LOG_LEVEL")
	}
	for _, field := range []struct {
		key string
		dst *time.Duration
		max time.Duration
	}{
		{"REQUEST_TIMEOUT", &c.RequestTimeout, 25 * time.Second},
		{"SHUTDOWN_TIMEOUT", &c.ShutdownTimeout, time.Minute},
	} {
		if v := e.Getenv(field.key); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 || d > field.max {
				return Config{}, fmt.Errorf("%s must be positive and at most %s", field.key, field.max)
			}
			*field.dst = d
		}
	}
	if v := e.Getenv("MAX_BODY_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 || n > 16<<20 {
			return Config{}, fmt.Errorf("MAX_BODY_BYTES must be between 1 and 16777216")
		}
		c.MaxBodyBytes = n
	}
	return c, nil
}
