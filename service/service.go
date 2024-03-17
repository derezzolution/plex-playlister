package service

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/derezzolution/plex-playlister/config"
	"github.com/derezzolution/plex-playlister/version"
)

type Service struct {
	Version *version.Version
	License string
	Config  *config.Config
}

func NewService(packageFS *embed.FS) *Service {
	config, err := config.NewConfig()
	if err != nil {
		log.Fatalf("error reading config: %v", err)
	}

	return &Service{
		Version: version.NewVersion(),
		License: readLicense(packageFS),
		Config:  config,
	}
}

func (s *Service) LogSummary() {
	s.Version.LogSummary()
	log.Printf("loading %s...\n\n%s\n", filepath.Base(os.Args[0]), s.License)
}

// readLicense reads the embedded LICENSE file.
func readLicense(packageFS *embed.FS) string {
	license, err := packageFS.ReadFile("LICENSE")
	if err != nil {
		log.Fatalf("error reading license: %v", err)
	}
	return string(license)
}
