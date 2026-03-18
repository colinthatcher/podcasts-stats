package internal

import "log/slog"

// Global state for the service file
var (
	store = NewStore()
)

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

func (s *Store) getEpisode(name string, id int) (*Item, bool) {
	pod, found := s.getPodcast(name)
	if !found {
		return nil, false
	}

	foundEpisode := pod.Channel.Items[id-1]
	if foundEpisode == nil {
		return nil, false
	}

	return foundEpisode, found
}

func NewStore() *Store {
	return &Store{
		podcasts: make(map[string]*Podcast),
	}
}
