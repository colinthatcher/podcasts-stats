package internal

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func GetRSSFeed(podcastName string, url string) ([]byte, error) {
	filename := fmt.Sprintf("%s.xml", podcastName)
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return getAndPersistFeed(filename, url)
	}
	hourAgo := time.Now().Add(-1 * time.Hour)
	if fileInfo.ModTime().After(hourAgo) {
		// return the cached file
		slog.Info("returning cached file", "filename", filename)
		return os.ReadFile(filename)
	}
	return getAndPersistFeed(filename, url)
}

func getAndPersistFeed(filename string, url string) ([]byte, error) {
	slog.Info("retrieving rss feed")
	// else re-retrieve the file from the rss feed
	content, err := fetchFeed(url)
	if err != nil {
		return nil, err
	}
	// persist the new content locally to avoid a ton of api calls
	err = os.WriteFile(filename, content, 0644)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func fetchFeed(url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode > 299 {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return body, nil
}
