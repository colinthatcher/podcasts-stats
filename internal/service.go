package internal

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"
)

type Podcast struct {
	XMLName xml.Name `xml:"rss"`
	Channel *Channel `xml:"channel"`
}

type Channel struct {
	XMLName xml.Name `xml:"channel"`
	Title   string   `xml:"title"`
	Items   []*Item  `xml:"item"`
}

type Item struct {
	XMLName        xml.Name `xml:"item"`
	Guid           string   `xml:"guid"`
	Title          string   `xml:"title"`
	PublishDateStr string   `xml:"pubDate"`
	DurationStr    string   `xml:"duration"`
	Description    string   `xml:"description"`
	Duration       time.Duration
	PublishDate    time.Time
}

// TODO: This should eventually become a real database
type Store struct {
	podcasts map[string]*Podcast
}

func (s *Store) addPodcast(podcast *Podcast) {
	if s.podcasts == nil {
		slog.Warn("failed to add podcast due to store be uninitialized")
		return
	}
	if podcast == nil || podcast.Channel == nil || podcast.Channel.Title == "" {
		slog.Warn("failed to add podcast to local store")
		return
	}
	s.podcasts[podcast.Channel.Title] = podcast
}

func (s *Store) getPodcast(name string) (*Podcast, bool) {
	podcast, found := s.podcasts[name]
	return podcast, found
}

func NewStore() *Store {
	return &Store{
		podcasts: make(map[string]*Podcast),
	}
}

// TODO: Global state for the service file
var (
	store = NewStore()
)

func init() {
	slog.Info("Falling into service init method")

	xmlFile, err := os.Open("internal/resources/episodes.xml")
	if err != nil {
		slog.Error("Failed to read local podcast file. err=", err.Error())
	}
	defer xmlFile.Close()
	byteValue, _ := ioutil.ReadAll(xmlFile)

	podcast := PodcastFromBytes(byteValue)
	slog.Debug("podcast", podcast.Channel.Title)
	store.addPodcast(podcast)

	slog.Info("Finished service init method")
}

func PodcastFromBytes(bytes []byte) *Podcast {
	var podcast Podcast
	err := xml.Unmarshal(bytes, &podcast)
	if err != nil {
		slog.Error("failed to unmarshal xml podcast file. err=", err.Error())
		return nil
	}

	// special parsing
	for _, item := range podcast.Channel.Items {
		// parse out proper duration
		parsedDuration := strings.Split(item.DurationStr, ":")
		seconds := parsedDuration[2]
		minutes := parsedDuration[1]
		hours := parsedDuration[0]
		duration, err := time.ParseDuration(fmt.Sprintf("%sh%sm%ss", hours, minutes, seconds))
		if err != nil {
			slog.Error("failed to parse podcast duration. err=", err.Error())
			return nil
		}
		item.Duration = duration

		// parse out time string
		parsedTime, err := time.Parse("Mon, 2 Jan 2006 15:04:05 +0000", item.PublishDateStr)
		if err != nil {
			slog.Error("failed to parse podcast publish date. err=", err.Error())
			return nil
		}
		item.PublishDate = parsedTime
	}

	// sort items by published date
	sort.Slice(podcast.Channel.Items, func(i int, j int) bool {
		episodeI := podcast.Channel.Items[i]
		episodeJ := podcast.Channel.Items[j]
		return episodeI.PublishDate.Before(episodeJ.PublishDate)
	})

	return &podcast
}

func GetPodcast(name string, start int, end int) (*Podcast, error) {
	podcast, found := store.getPodcast(name)
	if !found {
		return nil, fmt.Errorf("podcast not found")
	}
	podcast.Channel.Items = podcast.Channel.Items[start:end]
	return podcast, nil
}
