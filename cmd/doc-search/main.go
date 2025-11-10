package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/dntiontk/civic-code/pkg/downloader"
	"github.com/dntiontk/civic-code/pkg/scraper"
	"github.com/itlightning/dateparse"
)

var (
	yearFlag        int
	beforeFlag      string
	afterFlag       string
	meetingTypeFlag string
	docNameFlag     string
	downloadDirFlag string
	downloadWorkers int
	downloadFlag    bool
)

func main() {
	flag.IntVar(&yearFlag, "year", -1, "filter documents by year")
	flag.StringVar(&beforeFlag, "before", "", "filter documents before date")
	flag.StringVar(&afterFlag, "after", "", "filter documents after date")
	flag.StringVar(&meetingTypeFlag, "meetingType", "", "filter documents by meeting type")
	flag.StringVar(&docNameFlag, "docName", "", "filter documents with string in name")
	flag.StringVar(&downloadDirFlag, "downloadDir", "./downloads", "directory to store downloaded PDFs")
	flag.IntVar(&downloadWorkers, "concurrency", 4, "number of concurrent downloads")
	flag.BoolVar(&downloadFlag, "download", false, "download matching PDFs to disk")
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
	log.Printf("scraper: fetched %d documents before filtering", len(docs))
	for _, filter := range filters {
		docs = filter(docs)
	}
	log.Printf("scraper: %d documents match the provided filters", len(docs))

	var downloadErrors []string
	if downloadFlag && len(docs) > 0 {
		log.Printf("downloader: starting download of %d documents to %s with concurrency=%d", len(docs), downloadDirFlag, downloadWorkers)
		if downloadWorkers < 1 {
			downloadWorkers = 1
		}
		downloaded, err := downloader.DownloadDocuments(ctx, docs, downloadDirFlag, downloadWorkers)
		if downloaded != nil {
			docs = downloaded
		}
		if err != nil {
			log.Printf("download errors: %v", err)
			downloadErrors = append(downloadErrors, err.Error())
		} else {
			for _, doc := range docs {
				log.Printf("downloader: saved %s (%s)", doc.FileName, doc.Link)
			}
			log.Printf("downloader: completed download of %d documents", len(docs))
		}
	} else if !downloadFlag {
		log.Printf("downloader: skipping download (pass -download to enable)")
	}

	type Result struct {
		Len    int                `json:"len"`
		Items  []scraper.Document `json:"items"`
		Errors []string           `json:"errors,omitempty"`
	}

	res := &Result{
		Len:    len(docs),
		Items:  docs,
		Errors: downloadErrors,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	if err := enc.Encode(res); err != nil {
		log.Fatal(err)
	}
}
