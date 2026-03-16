package internal

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log/slog"
	"math"
	"os"
	"slices"
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
	Query  string `form:"query"`
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
	const trailer = "trailer"
	for i, item := range podcast.Channel.Items {
		if strings.ToLower(item.Title) == trailer {
			slog.Debug("Filtered out the trailer episode.")
			podcast.Channel.Items = append(podcast.Channel.Items[:i], podcast.Channel.Items[i+1:]...)
			break
		}
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
		foundItems := searchPodcastEpisodes(podcast, searchOpts)
		podcast.Channel.Items = foundItems
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

type Term struct {
	Value       string
	TargetField string
	Operation   string
	Negated     bool
}

func (t *Term) String() string {
	return fmt.Sprintf("Term(Value=%s TargetField=%s Operation=%s Negated=%t)", t.Value, t.TargetField, t.Operation, t.Negated)
}

func createTerm(parts []string) *Term {
	field := parts[0]
	value := parts[1]

	neg := false
	if field[0] == '-' {
		field = field[1:]
		neg = true
	}

	return &Term{
		Value:       value,
		TargetField: field,
		Negated:     neg,
	}

}

func searchPodcastEpisodes(podcast *Podcast, searchOpts *SearchOptions) []*Item {
	// Parse search terms
	rawTerms := strings.Split(strings.ToLower(searchOpts.Query), " ")
	terms := []*Term{}
	for _, term := range rawTerms {
		operation := ""
		var t *Term
		switch {
		case strings.Contains(term, ":"):
			operation = ":"
			parts := strings.Split(term, ":")
			if len(parts) != 2 {
				slog.Warn("got invalid search term", "term", term)
				return nil
			}
			t = createTerm(parts)
			t.Operation = operation
		default:
			t = &Term{
				Value: term,
			}
		}
		terms = append(terms, t)
	}
	slog.Info("parsed search terms", "terms", terms)

	// translate search terms against items list
	items := podcast.Channel.Items
	for _, term := range terms {
		items = searchEpisodesByTerm(term, items)
	}
	return items
}

func searchEpisodesByTerm(term *Term, items []*Item) []*Item {
	foundItems := []*Item{}
	for _, item := range items {
		switch term.TargetField {
		case "title":
			condition := strings.Contains(strings.ToLower(item.Title), strings.ToLower(term.Value))
			if (term.Negated && !condition) || (!term.Negated && condition) {
				foundItems = append(foundItems, item)
				continue
			}
		case "desc":
			condition := strings.Contains(strings.ToLower(item.Description), strings.ToLower(term.Value))
			if (term.Negated && !condition) || (!term.Negated && condition) {
				foundItems = append(foundItems, item)
				continue
			}
		default:
			// default all search across all text fields
			if strings.Contains(strings.ToLower(item.Title), strings.ToLower(term.Value)) {
				foundItems = append(foundItems, item)
				continue
			}
			if strings.Contains(strings.ToLower(item.Description), strings.ToLower(term.Value)) {
				foundItems = append(foundItems, item)
				continue
			}
		}
	}
	return foundItems
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

type PodcastStats struct {
	Year                 string
	NumEpisodes          int
	TotalDuration        time.Duration
	AvgDuration          time.Duration
	LongestEpisode       time.Duration
	LongestEpisodeTitle  string
	LongestEpisodeDate   time.Time
	LongestEpisodeId     int
	ShortestEpisode      time.Duration
	ShortestEpisodeTitle string
	ShortestEpisodeDate  time.Time
	ShortestEpisodeId    int
}

func GetPodcastStats(name string) ([]*PodcastStats, error) {
	podcast, found := store.getPodcast(name)
	if !found {
		return nil, fmt.Errorf("podcast not found")
	}

	// gather stats
	totalStats := &PodcastStats{
		Year:            "total",
		ShortestEpisode: math.MaxInt64,
	}
	stats := make(map[string]*PodcastStats)
	for _, item := range podcast.Channel.Items {
		totalStats.TotalDuration += item.Duration
		totalStats.NumEpisodes++
		if item.Duration > totalStats.LongestEpisode {
			totalStats.LongestEpisode = item.Duration
			totalStats.LongestEpisodeDate = item.PublishDate
			totalStats.LongestEpisodeTitle = item.Title
			totalStats.LongestEpisodeId = item.Id
		}
		if item.Duration < totalStats.ShortestEpisode {
			totalStats.ShortestEpisode = item.Duration
			totalStats.ShortestEpisodeDate = item.PublishDate
			totalStats.ShortestEpisodeTitle = item.Title
			totalStats.ShortestEpisodeId = item.Id
		}

		year := strconv.Itoa(item.PublishDate.Year())
		if yearStat, exists := stats[year]; exists {
			yearStat.NumEpisodes++
			yearStat.TotalDuration += item.Duration
			if item.Duration > yearStat.LongestEpisode {
				yearStat.LongestEpisode = item.Duration
				yearStat.LongestEpisodeDate = item.PublishDate
				yearStat.LongestEpisodeTitle = item.Title
				yearStat.LongestEpisodeId = item.Id
			}
			if item.Duration < yearStat.ShortestEpisode {
				yearStat.ShortestEpisode = item.Duration
				yearStat.ShortestEpisodeDate = item.PublishDate
				yearStat.ShortestEpisodeTitle = item.Title
				yearStat.ShortestEpisodeId = item.Id
			}

		} else {
			stats[year] = &PodcastStats{
				Year:            strconv.Itoa(item.PublishDate.Year()),
				NumEpisodes:     1,
				TotalDuration:   item.Duration,
				LongestEpisode:  item.Duration,
				ShortestEpisode: item.Duration,
			}
		}
	}
	stats["total"] = totalStats

	var years []string
	for _, v := range stats {
		// grab the year for sorting
		years = append(years, v.Year)

		// calc avg duration of episodes
		avg := int(v.TotalDuration.Seconds()) / v.NumEpisodes
		v.AvgDuration = time.Duration(avg) * time.Second
	}

	// Sort by total, and then newest year first
	slices.Sort(years)
	slices.Reverse(years)

	output := []*PodcastStats{}
	for _, year := range years {
		output = append(output, stats[year])
	}
	return output, nil
}
