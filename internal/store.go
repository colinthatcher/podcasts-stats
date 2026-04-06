package internal

import (
	"log/slog"
	"sort"
)

// Global state for the service file
var (
	store = NewStore()
)

type AvailablePodcastMetadata struct {
	Name    string
	FeedUrl string
}

var AvailablePodcasts = []*AvailablePodcastMetadata{
	{
		Name:    "Eagle Eye",
		FeedUrl: "https://feeds.simplecast.com/_ENNUG3a",
	},
	{
		Name:    "Phillies Talk",
		FeedUrl: "https://feeds.simplecast.com/GtcqZGk4",
	},
	{
		Name:    "Sixers Talk",
		FeedUrl: "https://feeds.simplecast.com/jxr32ewl",
	},
	{
		Name:    "Flyers Talk",
		FeedUrl: "https://feeds.simplecast.com/5lwfh_0s",
	},
	{
		Name:    "Takeoff with John Clark",
		FeedUrl: "https://feeds.simplecast.com/sx0cuZun",
	},
	{
		Name:    "Go Birds!",
		FeedUrl: "https://feeds.megaphone.fm/ENTDM6324343768",
	},
	{
		Name:    "New Heights",
		FeedUrl: "https://rss.art19.com/new-heights",
	},
}

type Store struct {
	podcasts map[string]*Podcast
}

func (s *Store) addPodcast(name string, podcast *Podcast) {
	if s.podcasts == nil {
		slog.Warn("failed to add podcast due to store be uninitialized")
		return
	}
	if podcast == nil || podcast.Channel == nil || podcast.Channel.Title == "" {
		slog.Warn("failed to add podcast to local store")
		return
	}
	s.podcasts[name] = podcast
}

func (s *Store) getPodcast(name string) (*Podcast, bool) {
	podcast, found := s.podcasts[name]
	return podcast, found
}

func (s *Store) getEpisode(name string, id int) (*Item, bool) {
	pod, found := s.getPodcast(name)
	if !found {
		return nil, false
	}

	i := sort.Search(len(pod.Channel.Items), func(i int) bool {
		return pod.Channel.Items[i].Id <= id
	})
	if i >= len(pod.Channel.Items) {
		return nil, false
	}
	return pod.Channel.Items[i], true
}

func NewStore() *Store {
	return &Store{
		podcasts: make(map[string]*Podcast),
	}
}
