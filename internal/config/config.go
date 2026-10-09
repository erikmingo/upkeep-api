// Package config loads the process configuration from environment variables.
package config

import "github.com/caarlos0/env/v11"

type Config struct {
	Port        int    `env:"PORT" envDefault:"8080"`
	DatabaseURL string `env:"DATABASE_URL,required,notEmpty"`
}

func Load() (Config, error) {
	return env.ParseAs[Config]()
}
