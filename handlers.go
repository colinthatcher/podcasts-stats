package main

import (
	"errors"
	"log/slog"
	"math"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/colinthatcher/podcast-stats/internal"
	"github.com/colinthatcher/podcast-stats/templates"
	"github.com/colinthatcher/podcast-stats/templates/pages"
	"github.com/colinthatcher/podcast-stats/templates/pages/podcasts"

	"github.com/gin-gonic/gin"
)

// Views

// indexViewHandler handles a view for the index page.
func IndexViewHandler(c *gin.Context) {
	c.Redirect(http.StatusPermanentRedirect, "/podcasts")
	return
}

func PodcastsViewHandler(c *gin.Context) {
	// get hard coded podcasts
	podcastNames := []string{}
	for _, podcast := range internal.AvailablePodcasts {
		podcastNames = append(podcastNames, podcast.Name)
	}
	pages := pages.Podcasts(podcastNames)
	template := templates.Layout(
		"Podcasts",
		nil,
		pages,
	)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, template); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

func PodcastEpisodesViewHandler(c *gin.Context) {
	podcastName := c.Param("name")
	// check if this is a search request
	var podcast *internal.Podcast
	var err error
	searchOpts := &internal.SearchOptions{}
	if err := c.ShouldBind(searchOpts); err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to parse search parameters.", "err", err)
	}

	slog.InfoContext(c.Request.Context(), "performing episode search with query.", "searchOpts", searchOpts)
	podcast, err = internal.GetPodcast(podcastName, searchOpts)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to find podcast.", "name", podcastName)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	slog.InfoContext(c.Request.Context(), "found podcast.", "name", podcastName)

	episodePage := podcasts.PodcastEpisodes(podcastName, podcast, searchOpts)
	podcastEpisodesTemplate := templates.Layout(
		"Podcast Episodes",
		nil,
		podcasts.PodcastBaseLayout(podcastName, podcasts.Episodes, episodePage, searchOpts),
	)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, podcastEpisodesTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

func PodcastEpisodeViewHandler(c *gin.Context) {
	podcastName := c.Param("name")
	episodeId := c.Param("id")
	episode, err := internal.GetPodcastEpisode(podcastName, episodeId)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to find podcast episode.", "id", episodeId)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	slog.InfoContext(c.Request.Context(), "found podcast episode.", "episode", episode)

	pages := podcasts.PodcastEpisode(episode)
	podcastEpisodesTemplate := templates.Layout(
		episode.Title,
		nil,
		podcasts.PodcastBaseLayout(podcastName, podcasts.Episodes, pages, nil),
	)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, podcastEpisodesTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

func PodcastStastsViewHandler(c *gin.Context) {
	podcastName := c.Param("name")
	searchOpts := &internal.SearchOptions{}
	if err := c.ShouldBind(searchOpts); err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to parse search parameters.", "err", err)
	}
	slog.InfoContext(c.Request.Context(), "gathering podcast stats", "searchOpts", searchOpts)

	// unset default search options to get all available episodes for the search criteria
	searchOpts.Start = 0
	searchOpts.Offset = math.MaxInt
	stats, err := internal.GetPodcastStats(podcastName, searchOpts)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to get podcast stats.", "name", podcastName)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	slog.InfoContext(c.Request.Context(), "found podcast.", "name", podcastName)

	pages := podcasts.PodcastStats(podcastName, stats, searchOpts)
	template := templates.Layout(
		"Podcast Stats",
		nil,
		podcasts.PodcastBaseLayout(podcastName, podcasts.Stats, pages, searchOpts),
	)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, template); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

// APIs

// showContentAPIHandler handles an API endpoint to show content.
func ShowContentAPIHandler(c *gin.Context) {
	// Check, if the current request has a 'HX-Request' header.
	// For more information, see https://htmx.org/docs/#request-headers
	if !htmx.IsHTMX(c.Request) {
		// If not, return HTTP 400 error.
		c.AbortWithError(http.StatusBadRequest, errors.New("non-htmx request"))
		return
	}

	// Write HTML content.
	c.Writer.Write([]byte("<p>🎉 Yes, <strong>htmx</strong> is ready to use! (<code>GET /api/hello-world</code>)</p>"))

	// Send htmx response.
	htmx.NewResponse().Write(c.Writer)
}
