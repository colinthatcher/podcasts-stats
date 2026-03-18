package internal

import (
	"fmt"
	"log/slog"
	"strings"
)

type SearchOptions struct {
	Query  string `form:"query"`
	Start  int    `form:"start,default=0"`
	Offset int    `form:"offset,default=20"`
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
