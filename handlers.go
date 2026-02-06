package main

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/colinthatcher/podcast-stats-webapp/internal"
	"github.com/colinthatcher/podcast-stats-webapp/templates"
	"github.com/colinthatcher/podcast-stats-webapp/templates/pages"

	"github.com/gin-gonic/gin"
)

const PODCAST_NAME = "Eagle Eye: A Philadelphia Eagles Podcast"

// Views

// indexViewHandler handles a view for the index page.
func IndexViewHandler(c *gin.Context) {

	// Define template meta tags.
	metaTags := pages.MetaTags(
		"gowebly, htmx example page, go with htmx",               // define meta keywords
		"Welcome to example! You're here because it worked out.", // define meta description
	)

	// Define template body content.
	bodyContent := pages.BodyContent(
		"Welcome to example!",                // define h1 text
		"You're here because it worked out.", // define p text
	)

	// Define template layout for index page.
	indexTemplate := templates.Layout(
		"Welcome to example!", // define title text
		metaTags,              // define meta tags
		bodyContent,           // define body content
	)

	// Render index page template.
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, indexTemplate); err != nil {
		// If not, return HTTP 500 error.
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

}

func PodcastEpisodesViewHandler(c *gin.Context) {
	// check if this is a search request
	var podcast *internal.Podcast
	var err error
	searchOpts := &internal.SearchOptions{}
	if err := c.ShouldBindQuery(searchOpts); err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to parse search parameters.", "err", err)
	}

	slog.InfoContext(c.Request.Context(), "performing episode search with query.", "searchOpts", searchOpts)
	podcast, err = internal.GetPodcast(PODCAST_NAME, searchOpts)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to find podcast.", "name", PODCAST_NAME)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	slog.InfoContext(c.Request.Context(), "found podcast.", "name", PODCAST_NAME)

	pages := pages.PodcastEpisodes(podcast, searchOpts)
	podcastEpisodesTemplate := templates.Layout(
		"Podcast Episodes",
		nil,
		pages,
	)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, podcastEpisodesTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

func PodcastEpisodeViewHandler(c *gin.Context) {
	episodeId := c.Param("id")
	episode, err := internal.GetPodcastEpisode(PODCAST_NAME, episodeId)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to find podcast episode.", "id", episodeId)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	slog.InfoContext(c.Request.Context(), "found podcast episode.", "episode", episode)

	pages := pages.PodcastEpisode(episode)
	podcastEpisodesTemplate := templates.Layout(
		episode.Title,
		nil,
		pages,
	)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, podcastEpisodesTemplate); err != nil {
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
