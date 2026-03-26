package internal

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type SearchOptions struct {
	Query  string `form:"query"`
	Start  int    `form:"start,default=0"`
	Offset int    `form:"offset,default=50"`
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

// valid search term operations
const EQUALS = "="
const GREATER_THAN = ">"
const LESS_THAN = "<"
const NEGATIVE = "-"

// valid field search terms
const TITLE = "title"
const DESCRIPTION = "description"
const DURATION = "duration"
const PUBLISH_DATE = "publish_date"

func createTerm(term string) *Term {
	operation := ""
	t := &Term{}
	switch {
	case strings.Contains(term, EQUALS):
		operation = EQUALS
	case strings.Contains(term, GREATER_THAN):
		// integer checks: durations, datetimes
		operation = GREATER_THAN
	case strings.Contains(term, LESS_THAN):
		// integer checks: durations, datetimes
		operation = LESS_THAN
	}

	parts := strings.Split(term, operation)

	if string(parts[0][0]) == NEGATIVE {
		if len(parts[0]) == 1 {
			parts = parts[1:]
		} else {
			parts[0] = parts[0][1:]
		}
		t.Negated = true
	}

	switch operation {
	case EQUALS:
		if parts[0] != TITLE &&
			parts[0] != DESCRIPTION &&
			parts[0] != DURATION &&
			parts[0] != PUBLISH_DATE {
			return nil
		}
		t.TargetField = parts[0]
		t.Value = parts[1]
		t.Operation = operation
	case GREATER_THAN, LESS_THAN:
		if parts[0] != DURATION && parts[0] != PUBLISH_DATE {
			return nil
		}
		t.TargetField = parts[0]
		t.Value = parts[1]
		t.Operation = operation
	default:
		t.Value = strings.Join(parts, "")
	}

	return t
}

func searchSplitQuery(searchOpts *SearchOptions) []string {
	splits := []string{}
	currSplit := ""
	splitIsQuote := false
	for _, char := range searchOpts.Query {
		// either seperator or space in quote
		if char == ' ' {
			// terminate term parsing
			if !splitIsQuote {
				if currSplit != "" {
					splits = append(splits, currSplit)
				}
				currSplit = ""
				splitIsQuote = false
				continue
			}
		}
		// either start or end of quoted string
		if char == '"' || char == '\'' {
			if splitIsQuote {
				// end
				splitIsQuote = false
			} else {
				// start
				splitIsQuote = true
			}
			continue
		}
		currSplit = currSplit + string(char)
	}

	// grab the last split if it exists
	if currSplit != "" {
		splits = append(splits, currSplit)
	}

	return splits
}

func searchPodcastEpisodes(podcast *Podcast, searchOpts *SearchOptions) []*Item {
	// Parse search terms
	// TODO: Doesn't work with the example query
	rawTerms := searchSplitQuery(searchOpts)
	slog.Info("checking search string splitting", "rawTerms", rawTerms)

	// translate string segments into terms
	terms := []*Term{}
	for _, term := range rawTerms {
		t := createTerm(term)
		if t != nil {
			terms = append(terms, t)
		}
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
		case TITLE:
			condition := strings.Contains(strings.ToLower(item.Title), strings.ToLower(term.Value))
			if (term.Negated && !condition) || (!term.Negated && condition) {
				foundItems = append(foundItems, item)
				continue
			}
		case DESCRIPTION:
			condition := strings.Contains(strings.ToLower(item.Description), strings.ToLower(term.Value))
			if (term.Negated && !condition) || (!term.Negated && condition) {
				foundItems = append(foundItems, item)
				continue
			}
		case DURATION:
			parsedDuration, err := time.ParseDuration(term.Value)
			if err != nil {
				slog.Warn("failed to parse search term duration", "value", term.Value, "err", err.Error())
				continue
			}
			var condition bool
			switch term.Operation {
			case EQUALS:
				condition = item.Duration == parsedDuration
			case GREATER_THAN:
				condition = item.Duration > parsedDuration
			case LESS_THAN:
				condition = item.Duration < parsedDuration
			default:
				continue
			}
			if (term.Negated && !condition) || (!term.Negated && condition) {
				foundItems = append(foundItems, item)
				continue
			}
		case PUBLISH_DATE:
			parsedTime, err := time.Parse(time.RFC3339, term.Value)
			if err != nil {
				slog.Warn("failed to parse search term time", "value", term.Value, "err", err.Error())
				continue
			}
			var condition bool
			switch term.Operation {
			case EQUALS:
				condition = item.PublishDate.Equal(parsedTime)
			case GREATER_THAN:
				condition = item.PublishDate.After(parsedTime)
			case LESS_THAN:
				condition = item.PublishDate.Before(parsedTime)
			default:
				continue
			}
			if (term.Negated && !condition) || (!term.Negated && condition) {
				foundItems = append(foundItems, item)
				continue
			}
		default:
			// default all search across all text fields
			titleCondition := strings.Contains(strings.ToLower(item.Title), strings.ToLower(term.Value))
			descCondition := strings.Contains(strings.ToLower(item.Description), strings.ToLower(term.Value))
			if (term.Negated && !titleCondition && !descCondition) || (!term.Negated && (titleCondition || descCondition)) {
				foundItems = append(foundItems, item)
				continue
			}
		}
	}
	return foundItems
}
