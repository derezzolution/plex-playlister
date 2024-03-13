package config

import (
	"embed"
	"encoding/json"
	"fmt"
)

type Config struct {
	PlexServerUrl string                     `json:"plexServerUrl"`
	PlexToken     string                     `json:"plexToken"`
	Playlists     map[string]*PlaylistConfig `json:"playlists"`
	HttpPort      int                        `json:"port"` // Http port
}

type PlaylistConfig struct {
	PlexRatingKey int `json:"plexRatingKey"` // Playlists to expose
}

func NewConfig(packageFS *embed.FS) (*Config, error) {
	configContent, err := packageFS.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("error reading embedded config: %s", err)
	}

	config := &Config{}
	err = json.Unmarshal(configContent, &config)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling embedded config: %s", err)
	}

	return config, nil
}
