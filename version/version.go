package version

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// These variables will be set by the linker at build time.
var (
	buildDateString          string
	versionShortHash         string
	versionHash              string
	versionBuildNumberString string
)

type Version struct {
	BuildDate          time.Time
	VersionShortHash   string
	VersionHash        string
	VersionBuildNumber int
}

func NewVersion() *Version {
	buildDate, _ := time.Parse("2006-01-02T15:04:05", buildDateString)
	versionBuildNumber, _ := strconv.Atoi(versionBuildNumberString)

	return &Version{
		BuildDate:          buildDate,
		VersionShortHash:   versionShortHash,
		VersionHash:        versionHash,
		VersionBuildNumber: versionBuildNumber,
	}
}

func (v *Version) LogSummary() {
	log.Printf("%s Build %d-%s (%s)", filepath.Base(os.Args[0]), v.VersionBuildNumber, v.VersionShortHash,
		v.BuildDate.Format("2006-01-02T15:04:05"))
}
