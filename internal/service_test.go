package internal

import (
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestPodcastDeepcopy(t *testing.T) {
	podcast := &Podcast{
		Channel: &Channel{
			Title: "Test Channel",
			Items: []*Item{
				{
					Id:             123,
					Guid:           "test_guid",
					Title:          "test item title",
					PublishDateStr: time.Now().String(),
					DurationStr:    (1 * time.Minute).String(),
					Description:    "test description",
					Duration:       1 * time.Minute,
					PublishDate:    time.Now(),
				},
				{
					Id:             456,
					Guid:           "test_guid",
					Title:          "test item title",
					PublishDateStr: time.Now().String(),
					DurationStr:    (2 * time.Minute).String(),
					Description:    "test description",
					Duration:       2 * time.Minute,
					PublishDate:    time.Now(),
				},
			},
		},
		Stats: &Stats{
			TotalItems: 2,
		},
	}
	tests := []struct {
		name          string
		podcast       *Podcast
		modifyPodcast func(p *Podcast)
		shouldEqual   bool
	}{
		{
			name:          "copies empty podcast",
			podcast:       &Podcast{},
			modifyPodcast: func(p *Podcast) {},
		},
		{
			name:          "copies podcast",
			podcast:       podcast,
			modifyPodcast: func(p *Podcast) {},
		},
		{
			name:    "copies podcast deeply, channel",
			podcast: podcast,
			modifyPodcast: func(p *Podcast) {
				p.Channel.Title = "new title"
			},
			shouldEqual: false,
		},
		{
			name:    "copies podcast deeply, items",
			podcast: podcast,
			modifyPodcast: func(p *Podcast) {
				for _, item := range p.Channel.Items {
					item.Description = "new description"
				}
			},
			shouldEqual: false,
		},
		{
			name:    "copies podcast deeply, stats",
			podcast: podcast,
			modifyPodcast: func(p *Podcast) {
				p.Stats.TotalItems = math.MaxInt
			},
			shouldEqual: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			copiedPodcast := podcast.deepcopy()
			tc.modifyPodcast(tc.podcast)
			if diff := cmp.Diff(podcast, copiedPodcast); diff != "" && tc.shouldEqual {
				t.Errorf("deepcopy() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
