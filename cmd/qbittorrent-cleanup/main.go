// Package main provides a utility to manage errored and missing-files
// torrents in qBittorrent.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	jobDeleteMissingFiles = "delete-missing-files"
	jobResumeErrored      = "resume-errored"
)

type Torrent struct {
	Hash  string `json:"hash"`
	State string `json:"state"`
	Name  string `json:"name"`
}

type QBitClient struct {
	BaseURL string
	HTTP    *http.Client
}

var allJobs = []string{jobDeleteMissingFiles, jobResumeErrored}

var rootCmd = &cobra.Command{
	Use:   "qbittorrent-cleanup",
	Short: "Removes missingFiles torrents and resumes recoverable errored torrents",
	Run:   runCleanup,
}

func init() {
	f := rootCmd.Flags()
	f.String("url", "", "qBittorrent WebUI URL")
	f.String("user", "", "qBittorrent username")
	f.String("password", "", "qBittorrent password")
	f.Duration("timeout", 30*time.Second, "API request timeout")
	f.StringSlice("job", nil, fmt.Sprintf("Jobs to run (repeatable: --job=%s --job=%s); defaults to both", jobDeleteMissingFiles, jobResumeErrored))

	cobra.OnInitialize(func() {
		viper.SetEnvPrefix("QBT")
		viper.AutomaticEnv()
		_ = viper.BindPFlags(f)
	})
}

func NewClient(baseURL string) (*QBitClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}
	return &QBitClient{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		HTTP:    &http.Client{Jar: jar},
	}, nil
}

func (c *QBitClient) Login(ctx context.Context, user, pass string) error {
	data := url.Values{"username": {user}, "password": {pass}}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.BaseURL+"/api/v2/auth/login",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *QBitClient) GetTorrents(ctx context.Context, filter string) ([]Torrent, error) {
	u := c.BaseURL + "/api/v2/torrents/info"
	if filter != "" {
		u += "?filter=" + url.QueryEscape(filter)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var torrents []Torrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, err
	}
	return torrents, nil
}

// GetErroredTorrents returns torrents in the errored state category.
func (c *QBitClient) GetErroredTorrents(ctx context.Context) ([]Torrent, error) {
	return c.GetTorrents(ctx, "errored")
}

// GetMissingFilesTorrents fetches all torrents and filters for those in the
// missingFiles state. This catches torrents that some qBittorrent versions
// exclude from the errored filter.
func (c *QBitClient) GetMissingFilesTorrents(ctx context.Context) ([]Torrent, error) {
	torrents, err := c.GetTorrents(ctx, "")
	if err != nil {
		return nil, err
	}
	var missing []Torrent
	for _, t := range torrents {
		if t.State == "missingFiles" {
			missing = append(missing, t)
		}
	}
	return missing, nil
}

func (c *QBitClient) DeleteTorrents(ctx context.Context, hashes []string) error {
	data := url.Values{"hashes": {strings.Join(hashes, "|")}, "deleteFiles": {"true"}}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.BaseURL+"/api/v2/torrents/delete",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func (c *QBitClient) Resume(ctx context.Context, hashes []string) error {
	data := url.Values{"hashes": {strings.Join(hashes, "|")}}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.BaseURL+"/api/v2/torrents/start",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCleanup(_ *cobra.Command, _ []string) {
	apiURL := viper.GetString("url")
	user := viper.GetString("user")
	pass := viper.GetString("password")
	timeout := viper.GetDuration("timeout")
	jobs := viper.GetStringSlice("job")

	if apiURL == "" || user == "" || pass == "" {
		log.Fatal("Error: Missing required configuration (URL, User, or Password)")
	}

	// Default: run all jobs when none explicitly selected.
	if len(jobs) == 0 {
		jobs = allJobs
	}

	client, err := NewClient(apiURL)
	if err != nil {
		log.Fatalf("Client error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := client.Login(ctx, user, pass); err != nil {
		fmt.Printf("Auth error: %v\n", err)
		return
	}

	for _, job := range jobs {
		switch job {
		case jobDeleteMissingFiles:
			deleteMissingFiles(ctx, client)
		case jobResumeErrored:
			resumeErrored(ctx, client)
		default:
			fmt.Printf("Unknown job: %s\n", job)
		}
	}
}

func deleteMissingFiles(ctx context.Context, client *QBitClient) {
	torrents, err := client.GetMissingFilesTorrents(ctx)
	if err != nil {
		fmt.Printf("Fetch error: %v\n", err)
		return
	}

	if len(torrents) == 0 {
		fmt.Println("No missing-files torrents found.")
		return
	}

	hashes := make([]string, len(torrents))
	for i, t := range torrents {
		fmt.Printf("Found missing files for: %s\n", t.Name)
		hashes[i] = t.Hash
	}

	fmt.Printf("Deleting %d torrents with missing files...\n", len(hashes))
	if err := client.DeleteTorrents(ctx, hashes); err != nil {
		log.Printf("Delete error: %v", err)
		return
	}
	fmt.Println("Delete-missing-files done.")
}

func resumeErrored(ctx context.Context, client *QBitClient) {
	torrents, err := client.GetErroredTorrents(ctx)
	if err != nil {
		fmt.Printf("Fetch error: %v\n", err)
		return
	}

	var toResume []string

	for _, t := range torrents {
		if t.State == "missingFiles" {
			// Handled by delete-missing-files job; skip here.
			continue
		}
		fmt.Printf("Resuming errored torrent: %s\n", t.Name)
		toResume = append(toResume, t.Hash)
	}

	if len(toResume) == 0 {
		fmt.Println("No recoverable errored torrents found.")
		return
	}

	fmt.Printf("Resuming %d recoverable torrents...\n", len(toResume))
	if err := client.Resume(ctx, toResume); err != nil {
		log.Printf("Resume error: %v", err)
		return
	}
	fmt.Println("Resume-errored done.")
}
