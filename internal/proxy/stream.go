package proxy

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/config"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/database"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/jellyfin"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/models"
)

type StreamProxy struct {
	db         *database.DB
	jf         *jellyfin.Client
	cfg        *config.Config
	httpClient *http.Client
}

func NewStreamProxy(db *database.DB, jf *jellyfin.Client, cfg *config.Config) *StreamProxy {
	return &StreamProxy{
		db:  db,
		jf:  jf,
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 0, // No timeout for streaming
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (p *StreamProxy) ServeStream(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		http.Error(w, "invalid session", http.StatusBadRequest)
		return
	}

	// Validate session
	session, err := p.db.GetSessionByID(r.Context(), sessionID)
	if err != nil || session == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if session.FinishedAt.Valid {
		http.Error(w, "session has ended", http.StatusForbidden)
		return
	}

	// Check if session is still active (heartbeat)
	if !session.IsActive(p.cfg.SessionHeartbeatTimeout) {
		p.db.FinishSession(r.Context(), sessionID, models.TerminationReasonTimeout)
		p.db.DecrementConcurrentViewers(r.Context(), session.ShareID)
		http.Error(w, "session timed out", http.StatusForbidden)
		return
	}

	// Get the share
	share, err := p.db.GetShareByID(r.Context(), session.ShareID)
	if err != nil || share == nil || !share.IsValid() {
		http.Error(w, "share not available", http.StatusForbidden)
		return
	}

	// Get the path after the session ID
	path := chi.URLParam(r, "*")
	if path == "" {
		path = "master.m3u8"
	}

	// Determine which item to stream
	// For Season shares, the itemId or MediaSourceId query param specifies the episode
	itemID := share.JellyfinItemID
	if episodeID := r.URL.Query().Get("itemId"); episodeID != "" {
		itemID = episodeID
	} else if mediaSourceID := r.URL.Query().Get("MediaSourceId"); mediaSourceID != "" {
		itemID = mediaSourceID
	}

	// MusicAlbum/Audio shares use Jellyfin's audio HLS endpoints.
	isAudio := share.ItemType == "MusicAlbum" || share.ItemType == "Audio"

	// Build Jellyfin URL
	jellyfinURL := p.buildJellyfinStreamURL(itemID, path, r.URL.RawQuery, isAudio)

	// Proxy the request
	p.proxyRequest(w, r, jellyfinURL)
}

func (p *StreamProxy) buildJellyfinStreamURL(itemID, path, query string, isAudio bool) string {
	baseURL := p.jf.BaseURL()

	// Parse existing query and ensure api_key is set (don't duplicate).
	params, _ := url.ParseQuery(query)

	key := p.jf.APIKey()
	if params.Get("api_key") == "" {
		params.Set("api_key", key)
	}

	if isAudio {
		// Do not forward jfshare-only routing parameters to Jellyfin.
		params.Del("itemId")
		params.Del("seasonId")
		params.Del("continue")

		// Album tracks use a browser-native progressive MP3 stream.
		// Jellyfin's /Audio/{itemId}/stream.{container} endpoint chooses
		// the requested output codec/container from the URL and parameters.
		if path == "stream.mp3" {
			params.Set("Static", "false")
			params.Set("mediaSourceId", itemID)
			params.Set("DeviceId", "jfshare-backend")
			params.Set("PlaySessionId", "jfshare-"+itemID)
			params.Set("AudioCodec", "mp3")
			params.Set("AudioBitrate", "192000")
			params.Set("MaxAudioChannels", "2")
			return baseURL + "/Audio/" + itemID + "/stream.mp3?" + params.Encode()
		}

		// Keep audio HLS as a fallback for any older/direct Audio URLs.
		if strings.HasSuffix(path, ".m3u8") {
			params.Set("AudioCodec", "aac")
			params.Set("AudioBitrate", "192000")
			params.Set("TranscodingMaxAudioChannels", "2")
			params.Set("AllowAudioStreamCopy", "false")
			params.Set("BreakOnNonKeyFrames", "True")
			params.Set("SegmentContainer", "ts")

			if path == "master.m3u8" {
				params.Set("MediaSourceId", itemID)
				params.Set("DeviceId", "jfshare-backend")
				params.Set("PlaySessionId", "jfshare-"+itemID)
				return baseURL + "/Audio/" + itemID + "/master.m3u8?" + params.Encode()
			}

			return baseURL + "/Audio/" + itemID + "/" + path + "?" + params.Encode()
		}

		if path != "" && path != "stream" {
			params.Del("AudioCodec")
			return baseURL + "/Audio/" + itemID + "/" + path + "?" + params.Encode()
		}

		params.Set("Static", "true")
		params.Set("mediaSourceId", itemID)
		return baseURL + "/Audio/" + itemID + "/stream?" + params.Encode()
	}

	// Video HLS: preserve the existing forced-transcoding behaviour.
	if strings.HasSuffix(path, ".m3u8") {
		params.Set("VideoCodec", "hevc")
		params.Set("MaxVideoBitrate", "1500000")
		params.Set("VideoBitrate", "1500000")
		params.Set("ToneMapping", "false")
		params.Set("EnableColorSpaceConversion", "false")
		params.Set("Profile", "main")
		params.Set("AllowVideoStreamCopy", "false")
		params.Set("AllowAudioStreamCopy", "false")
		params.Set("BreakOnNonKeyFrames", "True")
		params.Set("MaxMuxingQueueSize", "512")
		params.Set("MaxDelay", "5000000")
		params.Set("SegmentContainer", "mp4")

		if path == "master.m3u8" {
			params.Set("MediaSourceId", itemID)
			params.Set("DeviceId", "jfshare-backend")
			return baseURL + "/Videos/" + itemID + "/master.m3u8?" + params.Encode()
		}

		return baseURL + "/Videos/" + itemID + "/" + path + "?" + params.Encode()
	}

	if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".m4s") || strings.HasSuffix(path, ".mp4") {
		params.Del("AudioCodec")
		return baseURL + "/Videos/" + itemID + "/" + path + "?" + params.Encode()
	}

	// Generic video stream.
	params.Set("Static", "true")
	params.Set("mediaSourceId", itemID)
	return baseURL + "/Videos/" + itemID + "/stream?" + params.Encode()
}

func (p *StreamProxy) proxyRequest(w http.ResponseWriter, r *http.Request, targetURL string) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, nil)
	if err != nil {
		log.Printf("Failed to create proxy request: %v", err)
		http.Error(w, "proxy error", http.StatusBadGateway)
		return
	}

	// Copy relevant headers
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	if acceptHeader := r.Header.Get("Accept"); acceptHeader != "" {
		req.Header.Set("Accept", acceptHeader)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		log.Printf("Failed to proxy request to Jellyfin: %v", err)
		http.Error(w, "proxy error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		// Skip hop-by-hop headers
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set cache control for streaming content
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.WriteHeader(resp.StatusCode)

	// Stream the response
	io.Copy(w, resp.Body)
}

func isHopByHopHeader(header string) bool {
	hopByHopHeaders := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailers":            true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}
	return hopByHopHeaders[header]
}

// ServeImage proxies images from Jellyfin
func (p *StreamProxy) ServeImage(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	imageType := chi.URLParam(r, "type")

	share, err := p.db.GetShareByToken(r.Context(), token)
	if err != nil || share == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var imageURL string
	switch imageType {
	case "poster":
		imageURL = p.jf.GetPosterURL(share.JellyfinItemID)
	case "backdrop":
		imageURL = p.jf.GetBackdropURL(share.JellyfinItemID)
	case "logo":
		imageURL = p.jf.GetLogoURL(share.JellyfinItemID)
	case "thumb":
		imageURL = p.jf.GetThumbURL(share.JellyfinItemID)
	default:
		http.Error(w, "invalid image type", http.StatusBadRequest)
		return
	}

	// Add API key
	imageURL += "?api_key=" + p.jf.APIKey()

	// Add query params for sizing
	if maxWidth := r.URL.Query().Get("maxWidth"); maxWidth != "" {
		imageURL += "&maxWidth=" + maxWidth
	}
	if maxHeight := r.URL.Query().Get("maxHeight"); maxHeight != "" {
		imageURL += "&maxHeight=" + maxHeight
	}
	if quality := r.URL.Query().Get("quality"); quality != "" {
		imageURL += "&quality=" + quality
	}

	// Proxy with caching enabled
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, imageURL, nil)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		http.Error(w, "error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy headers
	for key, values := range resp.Header {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Enable caching for images
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
