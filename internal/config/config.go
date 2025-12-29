package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Repository RepoConfig  `json:"repository"`
	Local      LocalConfig `json:"local"`
	Sync       SyncConfig  `json:"sync,omitempty"`
}

type RepoConfig struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type LocalConfig struct {
	NextLocalID int `json:"next_local_id"`
}

type SyncConfig struct {
	LastFullPull *time.Time `json:"last_full_pull,omitempty"`
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
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config: %w", err)
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
