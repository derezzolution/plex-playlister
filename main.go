package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/derezzolution/go-plex-client"
	"github.com/gorilla/mux"

	"github.com/derezzolution/plex-playlister/config"
)

//go:embed templates config.json LICENSE
var packageFS embed.FS

//go:embed static
var staticPackageFS embed.FS

func main() {
	log.Printf("loading plex-playlister...\n\n%s\n", readLicense())
	config, err := config.NewConfig(&packageFS)
	if err != nil {
		log.Fatalf("error reading config: %v", err)
	}

	plexConnection, err := plex.New(config.PlexServerUrl, config.PlexToken)
	if err != nil {
		log.Fatalf("error connecting to plex server: %v", err)
	}

	mux := mux.NewRouter()
	mux.PathPrefix("/static").HandlerFunc(newStaticHandler())
	for _, plexRatingKey := range config.PlexRatingKeys {
		mux.HandleFunc(fmt.Sprintf("/playlist/%d", plexRatingKey), newPlaylistHandler(plexConnection, plexRatingKey))
		mux.HandleFunc(fmt.Sprintf("/playlist/%d/{index:[0-9]+}/thumb", plexRatingKey),
			newPlaylistThumbHandler(plexConnection, plexRatingKey))
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.HttpPort),
		Handler: mux,
	}

	// Create a channel to listen for interrupt or termination signals
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Start the server in a separate goroutine
	go func() {
		log.Printf("starting http service on *:%d", config.HttpPort)
		err = server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("fatal server error: %v", err)
		}
	}()

	// Wait for a signal
	<-interrupt

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shut down the server
	err = server.Shutdown(ctx)
	if err != nil {
		log.Fatalf("fatal server shutdown error: %v", err)
	}

	log.Println("http service shut down gracefully")
	os.Exit(0)
}

func newStaticHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		filename, _ := strings.CutPrefix(filepath.Clean(r.URL.Path), "/")

		file, err := staticPackageFS.Open(filename)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(filename)))
		io.Copy(w, file)
	}
}

func newPlaylistHandler(plexConnection *plex.Plex, plexRatingKey int) func(http.ResponseWriter, *http.Request) {

	type Track struct {
		Metadata      plex.Metadata // Metadata of tracks in the playlist
		MediaMetadata plex.Metadata // Media Metadata of tracks in playlist (e.g. IMDB id etc)
	}

	license := readLicense()
	playlistTemplate, err := template.New("playlist.html").Funcs(template.FuncMap{
		// Given a playlist track, format the episode code (e.g. sXXeYY).
		"formatEpisodeCode": func(metadata plex.Metadata) string {
			if metadata.ParentIndex == 0 && metadata.Index == 0 {
				return "" // Probably isn't s00e00
			}
			return fmt.Sprintf("s%0*de%0*d",
				2, metadata.ParentIndex, 2, metadata.Index)
		},

		// Extracts the IMDB id and constructs an IMDB URL.
		"formatImdbUrl": func(mediaMetadata plex.Metadata) string {
			for _, altGuid := range mediaMetadata.AltGUIDs {
				if strings.HasPrefix(altGuid.ID, "imdb://") {
					return fmt.Sprintf("https://www.imdb.com/title/%s", strings.TrimPrefix(altGuid.ID, "imdb://"))
				}
			}
			return "" // No IMDB ID
		},
	}).ParseFS(packageFS, "templates/playlist.html")
	if err != nil {
		log.Fatalf("error reading playlist template: %v", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {

		playlist, err := plexConnection.GetPlaylist(plexRatingKey)
		if err != nil {
			log.Printf("could not find playlist %d", plexRatingKey)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// One request for all items in the playlist, should we paginate this? Of course we should. Generally, though,
		// this whole endpoint should be cache on all fronts. Highly suggest putting this behind Cloudflare and caching
		// for hours+.
		playlistTrackRatingKeys := []string{}
		for _, metadata := range playlist.MediaContainer.Metadata {
			playlistTrackRatingKeys = append(playlistTrackRatingKeys, metadata.RatingKey)
		}
		metadata, err := plexConnection.GetMetadata(strings.Join(playlistTrackRatingKeys, ","))
		if err != nil {
			log.Printf("could not find metadata for rating keys %v: %v", playlistTrackRatingKeys, err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Merge records together so working with data in the template is much easier.
		trackMetadata := []Track{}
		for idx, m := range playlist.MediaContainer.Metadata {
			trackMetadata = append(trackMetadata, Track{
				Metadata:      m,
				MediaMetadata: metadata.MediaContainer.Metadata[idx],
			})
		}

		var renderedPlaylist strings.Builder
		err = playlistTemplate.Execute(&renderedPlaylist, struct {
			RatingKey string  // RatingKey of the playlist
			Size      int     // Number of tracks in the playlist
			Duration  int64   // Duration of the whole playlist
			Title     string  // Title of the playlist
			Tracks    []Track // Track metadata
		}{
			RatingKey: playlist.MediaContainer.RatingKey,
			Size:      playlist.MediaContainer.Size,
			Duration:  playlist.MediaContainer.Duration,
			Title:     playlist.MediaContainer.Title,
			Tracks:    trackMetadata,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		_, err = w.Write([]byte(fmt.Sprintf("<!--\n%s-->\n\n%s", license,
			renderedPlaylist.String())))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// createPlaylistThumbHandler creates a handler to proxy the thumbnail image. Use the plex rating key to keep things a
// touch obfuscated.
func newPlaylistThumbHandler(plexConnection *plex.Plex, plexRatingKey int) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Parse the index from the URL.
		vars := mux.Vars(r)
		index, err := strconv.Atoi(vars["index"])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// TODO - We shouldn't fetch the WHOLE playlist again. Look for another API and/or implement tighter requests in
		// the golang plex client library.
		playlist, err := plexConnection.GetPlaylist(plexRatingKey)
		if err != nil {
			log.Printf("could not find playlist %d", plexRatingKey)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Make sure we have a thumbnail URL.
		if index >= len(playlist.MediaContainer.Metadata) || len(playlist.MediaContainer.Metadata[index].Thumb) < 1 {
			http.Error(w, "Track Not Found", http.StatusNotFound)
			return
		}

		// Would be nice if plexConnection.GetThumbnail used .Thumb but it seems we need to reconstruct this. Fetch the
		// thumbnail.
		resp, err := plexConnection.GetThumbnail(playlist.MediaContainer.Metadata[index].RatingKey,
			strconv.Itoa(playlist.MediaContainer.Metadata[index].UpdatedAt)) //http.Response, error
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if resp.StatusCode != http.StatusOK {
			log.Printf("could not fetch thumbnail %s (statusCode=%d, status=%s, thumb=%s): ", r.URL.Path,
				resp.StatusCode, resp.Status, playlist.MediaContainer.Metadata[index].Thumb)
			http.Error(w, "Track Not Found", http.StatusNotFound)
			return
		}
		defer resp.Body.Close()

		io.Copy(w, resp.Body)
	}
}

// readLicense reads the embedded LICENSE file.
func readLicense() string {
	license, err := packageFS.ReadFile("LICENSE")
	if err != nil {
		log.Fatalf("error reading license: %v", err)
	}
	return string(license)
}
