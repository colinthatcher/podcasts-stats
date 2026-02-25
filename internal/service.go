package internal

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Podcast struct {
	XMLName xml.Name `xml:"rss"`
	Channel *Channel `xml:"channel"`
	Stats   *Stats
}

type Channel struct {
	XMLName xml.Name `xml:"channel"`
	Title   string   `xml:"title"`
	Items   []*Item  `xml:"item"`
}

type Item struct {
	XMLName        xml.Name `xml:"item"`
	Id             int
	Guid           string `xml:"guid"`
	Title          string `xml:"title"`
	PublishDateStr string `xml:"pubDate"`
	DurationStr    string `xml:"duration"`
	Description    string `xml:"description"`
	Duration       time.Duration
	PublishDate    time.Time
}

type Stats struct {
	TotalItems int
}

func (p *Podcast) deepcopy() *Podcast {
	copyPodcast := &Podcast{}
	if p.Channel != nil {
		copyPodcast.Channel = &Channel{}
		copyPodcast.Channel.Title = p.Channel.Title
		if len(p.Channel.Items) > 0 {
			copyPodcast.Channel.Items = []*Item{}
		}
	}
	if p.Stats != nil {
		copyPodcast.Stats = &Stats{}
		copyPodcast.Stats.TotalItems = p.Stats.TotalItems
	}

	for _, item := range p.Channel.Items {
		copyItem := &Item{}
		copyItem.Id = item.Id
		copyItem.Guid = item.Guid
		copyItem.Title = item.Title
		copyItem.PublishDateStr = item.PublishDateStr
		copyItem.DurationStr = item.DurationStr
		copyItem.Description = item.Description
		copyItem.Duration = item.Duration
		copyItem.PublishDate = item.PublishDate
		copyPodcast.Channel.Items = append(copyPodcast.Channel.Items, copyItem)
	}

	return copyPodcast
}

// TODO: This should eventually become a real database
type Store struct {
	podcasts map[string]*Podcast
}

type SearchOptions struct {
	Query  string `form:"search"`
	Start  int    `form:"start,default=0"`
	Offset int    `form:"offset,default=20"`
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

// TODO: Global state for the service file
var (
	store = NewStore()
)

func init() {
	slog.Info("Falling into service init method")

	xmlFile, err := os.Open("internal/resources/episodes.xml")
	if err != nil {
		slog.Error("Failed to read local podcast file.", "err", err.Error())
	}
	defer xmlFile.Close()
	byteValue, _ := ioutil.ReadAll(xmlFile)

	podcast := PodcastFromBytes(byteValue)
	store.addPodcast(podcast)

	slog.Info("Finished service init method")
}

func PodcastFromBytes(bytes []byte) *Podcast {
	var podcast Podcast
	err := xml.Unmarshal(bytes, &podcast)
	if err != nil {
		slog.Error("failed to unmarshal xml podcast file.", "err", err.Error())
		return nil
	}

	// filter out if the last episode is a "Trailer"
	// TODO: This can be improved by checking trailer against a lowercased title
	if strings.Contains(podcast.Channel.Items[len(podcast.Channel.Items)-1].Title, "Trailer") {
		podcast.Channel.Items = podcast.Channel.Items[:len(podcast.Channel.Items)-1]
	}

	// special parsing
	for i, item := range podcast.Channel.Items {
		// parse out proper duration
		parsedDuration := strings.Split(item.DurationStr, ":")
		seconds := parsedDuration[2]
		minutes := parsedDuration[1]
		hours := parsedDuration[0]
		duration, err := time.ParseDuration(fmt.Sprintf("%sh%sm%ss", hours, minutes, seconds))
		if err != nil {
			slog.Error("failed to parse podcast duration.", "err", err.Error())
			return nil
		}

		// parse out time string
		parsedTime, err := time.Parse("Mon, 2 Jan 2006 15:04:05 +0000", item.PublishDateStr)
		if err != nil {
			slog.Error("failed to parse podcast publish date.", "err", err.Error())
			return nil
		}

		item.Id = len(podcast.Channel.Items) - i
		item.Duration = duration
		item.PublishDate = parsedTime
	}

	// sort items by published date
	sort.Slice(podcast.Channel.Items, func(i int, j int) bool {
		episodeI := podcast.Channel.Items[i]
		episodeJ := podcast.Channel.Items[j]
		return episodeI.PublishDate.Before(episodeJ.PublishDate)
	})

	podcast.Stats = &Stats{}
	podcast.Stats.TotalItems = len(podcast.Channel.Items)

	return &podcast
}

// Find Podcast, paginate containing episodes
func GetPodcast(name string, searchOpts *SearchOptions) (*Podcast, error) {
	podcast, found := store.getPodcast(name)
	if !found {
		return nil, fmt.Errorf("podcast not found")
	}
	podcast = podcast.deepcopy()

	// filter episodes to display in the table
	if searchOpts.Query != "" {
		matchedItems := []*Item{}
		for _, item := range podcast.Channel.Items {
			if strings.Contains(item.Title, searchOpts.Query) {
				matchedItems = append(matchedItems, item)
			} else if strings.Contains(item.Description, searchOpts.Query) {
				matchedItems = append(matchedItems, item)
			}
		}
		podcast.Channel.Items = matchedItems
	}

	maxPage := len(podcast.Channel.Items)
	start := searchOpts.Start
	end := searchOpts.Start + searchOpts.Offset
	if searchOpts.Start > maxPage {
		start = maxPage
	}
	if end > maxPage {
		end = maxPage
	}
	slog.Info("", "start", start, "offset", end)
	podcast.Channel.Items = podcast.Channel.Items[start:end]
	podcast.Stats.TotalItems = maxPage

	return podcast, nil
}

func GetPodcastEpisode(name string, id string) (*Item, error) {
	// get all podcast episodes
	epId, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("failed to convert id to int")
	}
	episode, found := store.getEpisode(name, epId)
	if !found {
		return nil, fmt.Errorf("episode not found")
	}
	// then find spefic episode
	return episode, nil
}
