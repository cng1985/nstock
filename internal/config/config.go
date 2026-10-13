package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr            string
	DBPath          string
	WebDir          string
	MonitorInterval time.Duration
}

func Load() Config {
	sec := atoi(getenv("NSTOCK_MONITOR_INTERVAL", "20"), 20)
	if sec < 5 {
		sec = 5
	}
	return Config{
		Addr:            getenv("NSTOCK_ADDR", ":8080"),
		DBPath:          getenv("NSTOCK_DB", "data/nstock.db"),
		WebDir:          getenv("NSTOCK_WEB", "web/dist"),
		MonitorInterval: time.Duration(sec) * time.Second,
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func atoi(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
