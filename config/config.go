package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	PlexServerUrl string                     `json:"plexServerUrl"`
	PlexToken     string                     `json:"plexToken"`
	Playlists     map[string]*PlaylistConfig `json:"playlists"`
	KeyCacheSalt  string                     `json:"keyCacheSalt"`
	HttpPort      int                        `json:"port"` // Http port
}

type PlaylistConfig struct {
	PlexRatingKey  int  `json:"plexRatingKey"`  // Playlists to expose
	VisibleOnIndex bool `json:"visibleOnIndex"` // Whether the playlist should be visible on the index page
}

func NewConfig() (*Config, error) {
	configContent, err := os.ReadFile("config.json")
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = json.Unmarshal(configContent, &config)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %s", err)
	}

	return config, nil
}
