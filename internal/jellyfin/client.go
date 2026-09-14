package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	userID     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type User struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

// FetchAndSetUserID fetches the first available user and stores the ID
func (c *Client) FetchAndSetUserID(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodGet, "/Users", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to fetch users: %d: %s", resp.StatusCode, string(body))
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return fmt.Errorf("failed to decode users: %w", err)
	}

	if len(users) == 0 {
		return fmt.Errorf("no users found in Jellyfin")
	}

	c.userID = users[0].ID
	return nil
}

type ItemInfo struct {
	ID                string        `json:"Id"`
	Name              string        `json:"Name"`
	Overview          string        `json:"Overview,omitempty"`
	Taglines          []string      `json:"Taglines,omitempty"`
	Type              string        `json:"Type"`
	RunTimeTicks      int64         `json:"RunTimeTicks,omitempty"`
	ImageTags         ImageTags     `json:"ImageTags,omitempty"`
	BackdropImageTags []string      `json:"BackdropImageTags,omitempty"`
	SeriesName        string        `json:"SeriesName,omitempty"`
	SeasonName        string        `json:"SeasonName,omitempty"`
	IndexNumber       int           `json:"IndexNumber,omitempty"`
	ParentIndexNumber int           `json:"ParentIndexNumber,omitempty"`
	ProductionYear    int           `json:"ProductionYear,omitempty"`
	PremiereDate      string        `json:"PremiereDate,omitempty"`
	OfficialRating    string        `json:"OfficialRating,omitempty"`
	CommunityRating   float64       `json:"CommunityRating,omitempty"`
	CriticRating      int           `json:"CriticRating,omitempty"`
	Genres            []string      `json:"Genres,omitempty"`
	Studios           []StudioInfo  `json:"Studios,omitempty"`
	People            []PersonInfo  `json:"People,omitempty"`
	MediaSources      []MediaSource `json:"MediaSources,omitempty"`
	Width             int           `json:"Width,omitempty"`
	Height            int           `json:"Height,omitempty"`
}

type ImageTags struct {
	Primary string `json:"Primary,omitempty"`
	Logo    string `json:"Logo,omitempty"`
	Thumb   string `json:"Thumb,omitempty"`
}

type StudioInfo struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

type PersonInfo struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
	Role string `json:"Role,omitempty"`
	Type string `json:"Type"`
}

type MediaSource struct {
	ID                   string        `json:"Id"`
	Name                 string        `json:"Name,omitempty"`
	Container            string        `json:"Container,omitempty"`
	Size                 int64         `json:"Size,omitempty"`
	Bitrate              int           `json:"Bitrate,omitempty"`
	SupportsDirectPlay   bool          `json:"SupportsDirectPlay"`
	SupportsDirectStream bool          `json:"SupportsDirectStream"`
	SupportsTranscoding  bool          `json:"SupportsTranscoding"`
	MediaStreams         []MediaStream `json:"MediaStreams,omitempty"`
}

type MediaStream struct {
	Type         string `json:"Type"`
	Codec        string `json:"Codec,omitempty"`
	Width        int    `json:"Width,omitempty"`
	Height       int    `json:"Height,omitempty"`
	BitRate      int    `json:"BitRate,omitempty"`
	Channels     int    `json:"Channels,omitempty"`
	SampleRate   int    `json:"SampleRate,omitempty"`
	DisplayTitle string `json:"DisplayTitle,omitempty"`
}

type PlaybackInfo struct {
	MediaSources  []PlaybackMediaSource `json:"MediaSources"`
	PlaySessionId string                `json:"PlaySessionId"`
}

type PlaybackMediaSource struct {
	ID                   string `json:"Id"`
	Name                 string `json:"Name,omitempty"`
	Container            string `json:"Container,omitempty"`
	TranscodingUrl       string `json:"TranscodingUrl,omitempty"`
	DirectStreamUrl      string `json:"DirectStreamUrl,omitempty"`
	SupportsDirectPlay   bool   `json:"SupportsDirectPlay"`
	SupportsDirectStream bool   `json:"SupportsDirectStream"`
	SupportsTranscoding  bool   `json:"SupportsTranscoding"`
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	reqURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Emby-Token", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

func (c *Client) GetItemForUser(ctx context.Context, userID, itemID string) (*ItemInfo, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is empty")
	}

	path := fmt.Sprintf("/Users/%s/Items/%s", userID, itemID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jellyfin API returned %d for user %s item %s: %s", resp.StatusCode, userID, itemID, string(body))
	}

	var item ItemInfo
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &item, nil
}

func (c *Client) GetItem(ctx context.Context, itemID string) (*ItemInfo, error) {
	if c.userID == "" {
		return nil, fmt.Errorf("user ID not set - call FetchAndSetUserID first")
	}

	return c.GetItemForUser(ctx, c.userID, itemID)
}

func (c *Client) GetPlaybackInfo(ctx context.Context, itemID string) (*PlaybackInfo, error) {
	path := fmt.Sprintf("/Items/%s/PlaybackInfo", itemID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jellyfin API returned %d: %s", resp.StatusCode, string(body))
	}

	var info PlaybackInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &info, nil
}

func (c *Client) GetPosterURL(itemID string) string {
	return fmt.Sprintf("%s/Items/%s/Images/Primary", c.baseURL, itemID)
}

func (c *Client) GetBackdropURL(itemID string) string {
	return fmt.Sprintf("%s/Items/%s/Images/Backdrop", c.baseURL, itemID)
}

func (c *Client) GetLogoURL(itemID string) string {
	return fmt.Sprintf("%s/Items/%s/Images/Logo", c.baseURL, itemID)
}

func (c *Client) GetThumbURL(itemID string) string {
	return fmt.Sprintf("%s/Items/%s/Images/Thumb", c.baseURL, itemID)
}

func (c *Client) GetStreamURL(itemID string, mediaSourceID string, container string) string {
	params := url.Values{}
	params.Set("Static", "true")
	params.Set("mediaSourceId", mediaSourceID)
	params.Set("api_key", c.apiKey)

	return fmt.Sprintf("%s/Videos/%s/stream.%s?%s", c.baseURL, itemID, container, params.Encode())
}

func (c *Client) GetHLSStreamURL(itemID string, mediaSourceID string) string {
	params := url.Values{}
	params.Set("MediaSourceId", mediaSourceID)
	params.Set("api_key", c.apiKey)
	params.Set("DeviceId", "jfshare-backend")
	params.Set("PlaySessionId", "jfshare-"+itemID)

	params.Set("SegmentContainer", "mp4")

	return fmt.Sprintf("%s/Videos/%s/master.m3u8?%s", c.baseURL, itemID, params.Encode())
}

func (c *Client) GetTranscodedStreamURL(transcodingPath string) string {
	if strings.HasPrefix(transcodingPath, "/") {
		return c.baseURL + transcodingPath + "&api_key=" + c.apiKey
	}
	return c.baseURL + "/" + transcodingPath + "&api_key=" + c.apiKey
}

func (c *Client) VerifyConnection(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodGet, "/System/Info/Public", nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Jellyfin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jellyfin returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) APIKey() string {
	return c.apiKey
}

// TicksToSeconds converts Jellyfin runtime ticks to seconds
func TicksToSeconds(ticks int64) int64 {
	return ticks / 10000000
}

// EpisodeInfo contains basic info for an episode in a season
type EpisodeInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	IndexNumber    int    `json:"indexNumber"`
	Overview       string `json:"overview,omitempty"`
	RuntimeSeconds int64  `json:"runtimeSeconds,omitempty"`
	HasPoster      bool   `json:"hasPoster"`
	PremiereDate   string `json:"premiereDate,omitempty"`
}

// GetSeasonEpisodesForUser returns all episodes in a season for a specific Jellyfin user.
func (c *Client) GetSeasonEpisodesForUser(ctx context.Context, userID, seasonID string) ([]EpisodeInfo, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is empty")
	}

	params := url.Values{}
	params.Set("ParentId", seasonID)
	params.Set("SortBy", "IndexNumber")
	params.Set("SortOrder", "Ascending")

	path := fmt.Sprintf("/Users/%s/Items?%s", userID, params.Encode())
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jellyfin API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []ItemInfo `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	episodes := make([]EpisodeInfo, 0, len(result.Items))
	for _, item := range result.Items {
		if item.Type != "Episode" {
			continue
		}

		ep := EpisodeInfo{
			ID:           item.ID,
			Name:         item.Name,
			IndexNumber:  item.IndexNumber,
			Overview:     item.Overview,
			PremiereDate: item.PremiereDate,
			HasPoster:    item.ImageTags.Primary != "",
		}
		if item.RunTimeTicks > 0 {
			ep.RuntimeSeconds = TicksToSeconds(item.RunTimeTicks)
		}
		episodes = append(episodes, ep)
	}

	return episodes, nil
}

func (c *Client) GetSeasonEpisodes(ctx context.Context, seasonID string) ([]EpisodeInfo, error) {
	if c.userID == "" {
		return nil, fmt.Errorf("user ID not set - call FetchAndSetUserID first")
	}

	return c.GetSeasonEpisodesForUser(ctx, c.userID, seasonID)
}

// GetSeriesSeasonsForUser returns all seasons in a series for a specific Jellyfin user.
func (c *Client) GetSeriesSeasonsForUser(ctx context.Context, userID, seriesID string) ([]EpisodeInfo, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is empty")
	}

	params := url.Values{}
	params.Set("ParentId", seriesID)
	params.Set("SortBy", "IndexNumber")
	params.Set("SortOrder", "Ascending")

	path := fmt.Sprintf("/Users/%s/Items?%s", userID, params.Encode())
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jellyfin API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []ItemInfo `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	seasons := make([]EpisodeInfo, 0, len(result.Items))
	for _, item := range result.Items {
		if item.Type != "Season" {
			continue
		}

		season := EpisodeInfo{
			ID:          item.ID,
			Name:        item.Name,
			IndexNumber: item.IndexNumber,
			Overview:    item.Overview,
			HasPoster:   item.ImageTags.Primary != "",
		}
		seasons = append(seasons, season)
	}

	return seasons, nil
}

func (c *Client) GetSeriesSeasons(ctx context.Context, seriesID string) ([]EpisodeInfo, error) {
	if c.userID == "" {
		return nil, fmt.Errorf("user ID not set - call FetchAndSetUserID first")
	}

	return c.GetSeriesSeasonsForUser(ctx, c.userID, seriesID)
}

// GetAlbumTracksForUser returns all audio tracks in a music album for a specific Jellyfin user.
func (c *Client) GetAlbumTracksForUser(ctx context.Context, userID, albumID string) ([]EpisodeInfo, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is empty")
	}

	params := url.Values{}
	params.Set("ParentId", albumID)
	params.Set("IncludeItemTypes", "Audio")
	params.Set("Recursive", "false")
	params.Set("SortBy", "ParentIndexNumber,IndexNumber,SortName")
	params.Set("SortOrder", "Ascending")

	path := fmt.Sprintf("/Users/%s/Items?%s", userID, params.Encode())
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jellyfin API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []ItemInfo `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	tracks := make([]EpisodeInfo, 0, len(result.Items))
	for _, item := range result.Items {
		if item.Type != "Audio" {
			continue
		}

		track := EpisodeInfo{
			ID:          item.ID,
			Name:        item.Name,
			IndexNumber: item.IndexNumber,
			Overview:    item.Overview,
			HasPoster:   item.ImageTags.Primary != "",
		}
		if item.RunTimeTicks > 0 {
			track.RuntimeSeconds = TicksToSeconds(item.RunTimeTicks)
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

func (c *Client) GetAlbumTracks(ctx context.Context, albumID string) ([]EpisodeInfo, error) {
	if c.userID == "" {
		return nil, fmt.Errorf("user ID not set - call FetchAndSetUserID first")
	}

	return c.GetAlbumTracksForUser(ctx, c.userID, albumID)
}
