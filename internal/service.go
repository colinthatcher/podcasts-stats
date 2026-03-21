package internal

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"math"
	"os"
	"slices"
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

func init() {
	slog.Info("Falling into service init method")
	for _, podcast := range AvailablePodcasts {
		feedBytes, err := GetRSSFeed(podcast.Name, podcast.FeedUrl)
		if err != nil {
			slog.Error("failed to retrieve feed content", "podcastName", podcast.Name, "error", err)
			os.Exit(1)
		}

		parsedPodcast := PodcastFromBytes(feedBytes)
		store.addPodcast(podcast.Name, parsedPodcast)
	}
	slog.Info("Finished service init method")
}

func PeriodicallyFetchRSSFeed() {
	// start process to periodically fetch the rss feed
	rssFeedFetchTicker := time.NewTicker(59 * time.Minute)
	slog.Info("Starting background rss feed checker...")
	for {
		select {
		case <-rssFeedFetchTicker.C:
			slog.Info("checking for updates to podcast feed...")
			for _, podcast := range AvailablePodcasts {
				feedBytes, err := GetRSSFeed(podcast.Name, podcast.FeedUrl)
				if err != nil {
					slog.Error("failed to update podcast in ticker", "podcastName", podcast.Name, "error", err)
				}

				parsedPodcast := PodcastFromBytes(feedBytes)
				store.addPodcast(podcast.Name, parsedPodcast)
			}
		}
	}
}

func PodcastFromBytes(bytes []byte) *Podcast {
	var podcast Podcast
	err := xml.Unmarshal(bytes, &podcast)
	if err != nil {
		slog.Error("failed to unmarshal xml podcast file.", "err", err.Error())
		return nil
	}

	// special parsing
	const trailer = "trailer"
	for i, item := range podcast.Channel.Items {
		// filter out if the last episode is a "Trailer"
		if strings.ToLower(item.Title) == trailer {
			slog.Debug("Filtered out the trailer episode.")
			podcast.Channel.Items = append(podcast.Channel.Items[:i], podcast.Channel.Items[i+1:]...)
			continue
		}

		// parse out proper duration
		var duration time.Duration
		if strings.Contains(item.DurationStr, ":") {
			parsedDuration := strings.Split(item.DurationStr, ":")
			seconds := parsedDuration[2]
			minutes := parsedDuration[1]
			hours := parsedDuration[0]
			duration, err = time.ParseDuration(fmt.Sprintf("%sh%sm%ss", hours, minutes, seconds))
			if err != nil {
				slog.Error("failed to parse podcast duration as timestamp", "err", err)
				return nil
			}
		} else {
			// assume duration string is number of seconds an episode is
			seconds, err := strconv.Atoi(item.DurationStr)
			if err != nil {
				slog.Error("failed to parse podcast duration as seconds to int", "err", err)
				return nil
			}
			duration, err = time.ParseDuration(fmt.Sprintf("%ds", seconds))
			if err != nil {
				slog.Error("failed to parse podcast duration as seconds", "err", err)
				return nil
			}
		}

		// parse out time string
		parseString := "Mon, 2 Jan 2006 15:04:05 +0000"
		if strings.Contains(item.PublishDateStr, "-") {
			parseString = "Mon, 2 Jan 2006 15:04:05 -0000"
		}
		parsedTime, err := time.Parse(parseString, item.PublishDateStr)
		if err != nil {
			slog.Error("failed to parse podcast publish date.", "err", err.Error())
			return nil
		}

		item.Id = len(podcast.Channel.Items) - i - 1
		item.Duration = duration
		item.PublishDate = parsedTime
	}

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
