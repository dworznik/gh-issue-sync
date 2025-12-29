package config

import (
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Repository RepoConfig  `toml:"repository"`
	Local      LocalConfig `toml:"local"`
	Sync       SyncConfig  `toml:"sync"`
}

type RepoConfig struct {
	Owner string `toml:"owner"`
	Repo  string `toml:"repo"`
}

type LocalConfig struct {
	NextLocalID int `toml:"next_local_id"`
}

type SyncConfig struct {
	LastFullPull *time.Time `toml:"last_full_pull"`
}

func Default(owner, repo string) Config {
	return Config{
		Repository: RepoConfig{Owner: owner, Repo: repo},
		Local:      LocalConfig{NextLocalID: 1},
	}
}

func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
