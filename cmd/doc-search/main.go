package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/dntiontk/civic-code/pkg/scraper"
	"github.com/itlightning/dateparse"
	"log"
	"os"
	"time"
)

var (
	yearFlag        int
	beforeFlag      string
	afterFlag       string
	meetingTypeFlag string
	docNameFlag     string
)

func main() {
	flag.IntVar(&yearFlag, "year", -1, "filter documents by year")
	flag.StringVar(&beforeFlag, "before", "", "filter documents before date")
	flag.StringVar(&afterFlag, "after", "", "filter documents after date")
	flag.StringVar(&meetingTypeFlag, "meetingType", "", "filter documents by meeting type")
	flag.StringVar(&docNameFlag, "docName", "", "filter documents with string in name")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	filters := make([]scraper.FilterFunc, 0)
	if yearFlag != -1 {
		filters = append(filters, scraper.ByYear(yearFlag))
	}
	if beforeFlag != "" {
		before, err := dateparse.ParseAny(beforeFlag)
		if err != nil {
			log.Fatal(err)
		}
		filters = append(filters, scraper.Before(before))
	}

	if afterFlag != "" {
		after, err := dateparse.ParseAny(afterFlag)
		if err != nil {
			log.Fatal(err)
		}
		filters = append(filters, scraper.After(after))
	}

	if meetingTypeFlag != "" {
		meetingType := scraper.GetMeetingType(meetingTypeFlag)
		filters = append(filters, scraper.ByMeetingType(meetingType))
	}

	if docNameFlag != "" {
		filters = append(filters, scraper.ByStringInName(docNameFlag))
	}

	docs, err := scraper.GetDocuments(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, filter := range filters {
		docs = filter(docs)
	}

	type Result struct {
		Len   int                `json:"len"`
		Items []scraper.Document `json:"items"`
	}

	res := &Result{
		Len:   len(docs),
		Items: docs,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	if err := enc.Encode(res); err != nil {
		log.Fatal(err)
	}
}
