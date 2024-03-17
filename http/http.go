package http

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/derezzolution/go-plex-client"
	"github.com/gorilla/mux"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"

	"github.com/derezzolution/plex-playlister/config"
	"github.com/derezzolution/plex-playlister/keycache"
	"github.com/derezzolution/plex-playlister/service"
)

type HttpService struct {
	service        *service.Service
	mux            *mux.Router
	server         *http.Server
	plexConnection *plex.Plex
	keyCache       *keycache.KeyCache
}

func NewHttpService(service *service.Service) *HttpService {
	mux := mux.NewRouter()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", service.Config.HttpPort),
		Handler: mux,
	}

	plexConnection, err := plex.New(service.Config.PlexServerUrl, service.Config.PlexToken)
	if err != nil {
		log.Fatalf("error creating new plex server connection: %v", err)
	}

	return &HttpService{
		service:        service,
		mux:            mux,
		server:         server,
		plexConnection: plexConnection,
		keyCache:       keycache.NewKeyCache(service.Config.KeyCacheSalt),
	}
}

func (h *HttpService) Start() {
	h.mux.HandleFunc("/", h.newIndexHandler())
	h.mux.PathPrefix("/static").HandlerFunc(h.newStaticHandler())
	for key := range h.service.Config.Playlists {
		h.mux.HandleFunc(fmt.Sprintf("/playlist/%s", key), h.newPlaylistHandler(key))
	}
	h.mux.HandleFunc("/playlist/thumb/{obfusKey:[0-9]+}", h.newPlaylistThumbHandler())

	// Start the server in a separate goroutine
	go func() {
		log.Printf("starting http service on *:%d", h.service.Config.HttpPort)
		err := h.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("fatal server error: %v", err)
		}
	}()
}

func (h *HttpService) Stop(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

func (h *HttpService) newStaticHandler() func(http.ResponseWriter, *http.Request) {
	m := newMinify()
	return func(w http.ResponseWriter, r *http.Request) {
		filename, _ := strings.CutPrefix(filepath.Clean(r.URL.Path), "/")

		if !strings.HasPrefix(filename, "static/") {
			log.Printf("error cannot open %s (it's only safe to open files in static/)", filename)
			http.NotFound(w, r)
			return
		}

		file, err := os.Open(filename)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		mediaType := mime.TypeByExtension(filepath.Ext(filename))
		w.Header().Set("Content-Type", mediaType)

		err = m.Minify(mediaType, w, file)
		if err != nil {
			io.Copy(w, file) // Return the regular file, if we can't minify
		}
	}
}

// newIndexHandler creates a handler for a playlist index page.
func (h *HttpService) newIndexHandler() func(http.ResponseWriter, *http.Request) {
	playlistTemplate, err := template.New("index.html").Funcs(templateFuncMap()).
		ParseFiles("templates/index.html")
	if err != nil {
		log.Fatalf("error reading playlist template: %v", err)
	}

	m := newMinify()
	return func(w http.ResponseWriter, r *http.Request) {

		type Playlist struct {
			PlaylistKey    string
			PlaylistConfig *config.PlaylistConfig
			MediaContainer plex.MediaContainer
		}

		// TODO - Parallelize (should fetch in chunks of 5 for a fatter pipe)
		playlists := []Playlist{}
		for playlistKey, playlistConfig := range h.service.Config.Playlists {
			playlist, err := h.plexConnection.GetPlaylist(playlistConfig.PlexRatingKey)
			if err != nil {
				log.Printf("could not find playlist %d", playlistConfig.PlexRatingKey)
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			playlists = append(playlists, Playlist{
				PlaylistKey:    playlistKey,
				PlaylistConfig: playlistConfig,
				MediaContainer: playlist.MediaContainer,
			})
		}
		sort.Slice(playlists, func(i, j int) bool {
			return playlists[i].PlaylistKey < playlists[j].PlaylistKey
		})

		content := strings.Builder{}
		err = playlistTemplate.Execute(&content, struct {
			Playlists []Playlist
		}{
			Playlists: playlists,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		renderHtml(h.service.License, m, content, w)
	}
}

func (h *HttpService) newPlaylistHandler(key string) func(http.ResponseWriter, *http.Request) {

	type Track struct {
		ObfuscatedKey      string        // String mapping to obfuscated track key
		ObfuscatedThumbKey string        // String mapping to obfuscated thumb key
		Metadata           plex.Metadata // Metadata of tracks in the playlist
		MediaMetadata      plex.Metadata // Media Metadata of tracks in playlist (e.g. IMDB id etc)
	}

	playlistTemplate, err := template.New("playlist.html").Funcs(templateFuncMap()).
		ParseFiles("templates/playlist.html")
	if err != nil {
		log.Fatalf("error reading playlist template: %v", err)
	}

	m := newMinify()
	return func(w http.ResponseWriter, r *http.Request) {

		playlist, err := h.plexConnection.GetPlaylist(h.service.Config.Playlists[key].PlexRatingKey)
		if err != nil {
			log.Printf("could not find playlist %d", h.service.Config.Playlists[key].PlexRatingKey)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// One request for all items in the playlist, should we paginate this? Of course we should. Generally, though,
		// this whole endpoint should be cached on all fronts. Highly suggest putting this behind Cloudflare and caching
		// for hours+.
		playlistTrackRatingKeys := []string{}
		for _, metadata := range playlist.MediaContainer.Metadata {
			playlistTrackRatingKeys = append(playlistTrackRatingKeys, metadata.RatingKey)
		}
		metadata, err := h.plexConnection.GetMetadata(strings.Join(playlistTrackRatingKeys, ","))
		if err != nil {
			log.Printf("could not find metadata for rating keys %v: %v", playlistTrackRatingKeys, err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Merge records together so working with data in the template is much easier.
		trackMetadata := []Track{}
		for idx, m := range playlist.MediaContainer.Metadata {
			trackMetadata = append(trackMetadata, Track{
				ObfuscatedKey:      h.keyCache.GetObfusKey(m.Key),
				ObfuscatedThumbKey: h.keyCache.GetObfusKey(m.Thumb),
				Metadata:           m,
				MediaMetadata:      metadata.MediaContainer.Metadata[idx],
			})
		}

		content := strings.Builder{}
		err = playlistTemplate.Execute(&content, struct {
			Key       string  // Key according to the config.PlaylistConfig
			RatingKey string  // RatingKey of the playlist
			Size      int     // Number of tracks in the playlist
			Duration  int64   // Duration of the whole playlist
			Title     string  // Title of the playlist
			Tracks    []Track // Track metadata
		}{
			Key:       key,
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

		renderHtml(h.service.License, m, content, w)
	}
}

// newPlaylistThumbHandler creates a handler to proxy the thumbnail image.
func (h *HttpService) newPlaylistThumbHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Grab the key from the key cache (e.g. obfusKey := /library/metadata/10122/thumb/1705219286).
		vars := mux.Vars(r)
		thumbChunks := strings.Split(strings.TrimPrefix(h.keyCache.GetKey(vars["obfusKey"]), "/"), "/")
		if len(thumbChunks) != 5 {
			http.Error(w, "Track Not Found", http.StatusNotFound)
			return
		}
		key := thumbChunks[2]     // e.g. 10122
		thumbID := thumbChunks[4] // e.g. 1705219286

		// Grab the thumbnail
		resp, err := h.plexConnection.GetThumbnail(key, thumbID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if resp.StatusCode != http.StatusOK {
			log.Printf("could not fetch thumbnail %s (statusCode=%d, status=%s, key=%s, thumbID=%s): ", r.URL.Path,
				resp.StatusCode, resp.Status, key, thumbID)
			http.Error(w, "Track Not Found", http.StatusNotFound)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Cache-Control", "max-age=2629800") // Cache for 1 month
		io.Copy(w, resp.Body)
	}
}

// templateFuncMap lists function helpers for template generation.
func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		// Given a playlist track, format the episode code (e.g. SXX.EYY).
		"formatEpisodeCode": func(metadata plex.Metadata) string {
			if metadata.ParentIndex == 0 && metadata.Index == 0 {
				return "" // Probably isn't S0.E0
			}
			return fmt.Sprintf("S%0*d.E%0*d",
				1, metadata.ParentIndex, 1, metadata.Index)
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

		// Increment the given integer by one.
		"increment": func(i int) int {
			return i + 1
		},
	}
}

// newMinify builds and configures a new minify.
func newMinify() *minify.M {
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.Add("text/html", &html.Minifier{
		KeepComments:     true,
		KeepDocumentTags: true,
		KeepEndTags:      true,
		KeepQuotes:       true,
	})
	m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	return m
}

// renderHtml renders the html content by injecting the license and minifying the response before writing it to the http
// response writer.
func renderHtml(license string, m *minify.M, content strings.Builder, w http.ResponseWriter) {
	mediaType := "text/html"
	minifiedPage, err := m.String(mediaType, fmt.Sprintf("<!--\n%s-->\n\n%s", license,
		content.String()))
	if err != nil {
		log.Printf("could not minify page: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", mediaType)
	_, err = w.Write([]byte(minifiedPage))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
