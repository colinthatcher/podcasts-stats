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
	rssFeedFetchTicker := time.NewTicker(61 * time.Minute)
	slog.Info("Starting background rss feed checker to run every 61 minutes...")
	for range rssFeedFetchTicker.C {
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

func PodcastFromBytes(bytes []byte) *Podcast {
	var podcast Podcast
	err := xml.Unmarshal(bytes, &podcast)
	if err != nil {
		slog.Error("failed to unmarshal xml podcast file.", "err", err.Error())
		return nil
	}

	// cleanup episodes
	for i, item := range podcast.Channel.Items {
		if strings.Contains(strings.ToLower(item.Title), "trailer") {
			podcast.Channel.Items = append(podcast.Channel.Items[:i], podcast.Channel.Items[i+1:]...)
		}
	}

	// special parsing
	for i, item := range podcast.Channel.Items {
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

		item.Id = len(podcast.Channel.Items) - i
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

type PodcastStatsDayOfTheWeek struct {
	NumEpisodes   int
	TotalDuration time.Duration
	AvgDuration   time.Duration
}

type PodcastStats struct {
	Year                   string
	NumEpisodes            int
	TotalDuration          time.Duration
	AvgDuration            time.Duration
	LongestEpisode         time.Duration
	LongestEpisodeTitle    string
	LongestEpisodeDate     time.Time
	LongestEpisodeId       int
	ShortestEpisode        time.Duration
	ShortestEpisodeTitle   string
	ShortestEpisodeDate    time.Time
	ShortestEpisodeId      int
	EpisodesPerDayOrder    []string
	EpisodesPerDay         map[string]int // displays github style timeline of podcasts per day
	StatsPerDayOfTheWeek   map[time.Weekday]*PodcastStatsDayOfTheWeek
	AvgDaysBetweenEpisodes time.Duration
}

func (p *PodcastStats) saveLongestEpisode(item *Item) {
	p.LongestEpisode = item.Duration
	p.LongestEpisodeDate = item.PublishDate
	p.LongestEpisodeTitle = item.Title
	p.LongestEpisodeId = item.Id
}

func (p *PodcastStats) saveShortestEpisode(item *Item) {
	p.ShortestEpisode = item.Duration
	p.ShortestEpisodeDate = item.PublishDate
	p.ShortestEpisodeTitle = item.Title
	p.ShortestEpisodeId = item.Id
}

func (p *PodcastStats) gatherItemStats(item *Item) {
	p.NumEpisodes++
	p.TotalDuration += item.Duration
	if item.Duration > p.LongestEpisode {
		p.saveLongestEpisode(item)
	}
	if item.Duration < p.ShortestEpisode {
		p.saveShortestEpisode(item)
	}
	if _, exists := p.EpisodesPerDay[item.PublishDate.Format("2006-01-02")]; exists {
		p.EpisodesPerDay[item.PublishDate.Format("2006-01-02")]++
	}
	p.StatsPerDayOfTheWeek[item.PublishDate.Weekday()].NumEpisodes++
	p.StatsPerDayOfTheWeek[item.PublishDate.Weekday()].TotalDuration += item.Duration
}

func NewPodcastStat(year int, isTotal bool) *PodcastStats {
	ps := &PodcastStats{
		ShortestEpisode:      math.MaxInt64,
		StatsPerDayOfTheWeek: map[time.Weekday]*PodcastStatsDayOfTheWeek{},
	}
	if year == 0 && isTotal {
		ps.Year = "total"
	} else {
		ps.Year = strconv.Itoa(year)
		ps.EpisodesPerDayOrder = getDaysInYear(year)
		emptyEpisodesPerDay := map[string]int{}
		for _, day := range ps.EpisodesPerDayOrder {
			emptyEpisodesPerDay[day] = 0
		}
		ps.EpisodesPerDay = emptyEpisodesPerDay
	}

	for _, day := range []time.Weekday{
		time.Sunday, time.Monday, time.Tuesday,
		time.Wednesday, time.Thursday, time.Friday, time.Saturday,
	} {
		weekday := &PodcastStatsDayOfTheWeek{}
		ps.StatsPerDayOfTheWeek[day] = weekday
	}

	return ps
}

// TODO: Fix year frequency of podcasts
func GetPodcastStats(name string, searchOpts *SearchOptions) ([]*PodcastStats, error) {
	podcast, err := GetPodcast(name, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("podcast not found. err=%v", err.Error())
	}

	// gather stats
	totalStats := NewPodcastStat(0, true)
	stats := make(map[string]*PodcastStats)
	for _, item := range podcast.Channel.Items {
		totalStats.gatherItemStats(item)

		year := strconv.Itoa(item.PublishDate.Year())
		if yearStat, exists := stats[year]; exists {
			yearStat.gatherItemStats(item)
		} else {
			yearStat := NewPodcastStat(item.PublishDate.Year(), false)
			yearStat.gatherItemStats(item)
			stats[year] = yearStat
		}
	}
	stats["total"] = totalStats

	var years []string
	for y, v := range stats {
		// grab the year for sorting
		years = append(years, v.Year)

		// calc avg duration of episodes
		avg := int(v.TotalDuration.Seconds()) / v.NumEpisodes
		v.AvgDuration = time.Duration(avg) * time.Second

		// calc avg duration of weekdays
		for _, weekday := range stats[y].StatsPerDayOfTheWeek {
			if weekday.NumEpisodes == 0 {
				continue
			}

			weekdayAvg := int(weekday.TotalDuration.Seconds()) / weekday.NumEpisodes
			weekday.AvgDuration = time.Duration(weekdayAvg) * time.Second
			slog.Info("debugging stats", "stats", weekday)
		}
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

// get an array strings matching every day in a given year
func getDaysInYear(year int) []string {
	days := make([]string, 0)
	for month := 1; month <= 12; month++ {
		for day := 1; day <= 31; day++ {
			switch month {
			case 2:
				// February
				if isLeapYear(year) && day == 30 {
					continue
				} else if !isLeapYear(year) && day == 29 {
					continue
				} else {
					days = append(days, fmt.Sprintf("%d-%02d-%02d", year, month, day))
				}
			case 4, 6, 9, 11:
				// 30 day months - April, June, September, November
				if day == 31 {
					continue
				}
				days = append(days, fmt.Sprintf("%d-%02d-%02d", year, month, day))
			default:
				// 31 day months
				days = append(days, fmt.Sprintf("%d-%02d-%02d", year, month, day))
			}
		}
	}
	return days
}

func isLeapYear(year int) bool {
	return year%4 == 0 && year%100 != 0 || year%400 == 0
}
