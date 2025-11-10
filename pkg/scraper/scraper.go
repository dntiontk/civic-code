package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// MeetingType defines a meeting with associated search terms
type MeetingType struct {
	Code        string `json:"code,omitempty"`
	Name        string `json:"name,omitempty"`
	SearchTerms []string
}

var (
	CC      = MeetingType{Code: "CC", Name: "City Council", SearchTerms: []string{"citycouncil", "citycouncilmeeting"}}
	DHSC    = MeetingType{Code: "DHSC", Name: "Development & Heritage Standing Committee", SearchTerms: []string{"developmentheritagestandingcommittee", "developmentheritagestandingcommitteemeeting"}}
	Special = MeetingType{Code: "Special", Name: "Special Meeting of Council", SearchTerms: []string{"specialmeetingofcouncil", "specialmeetingofcitycouncil", "specialcitycouncilmeeting", "specialincamerameetingofcouncil", "specialincamerameetingofcitycouncil"}}
	ETP     = MeetingType{Code: "ETP", Name: "Environment, Transportation & Public Safety Standing Committee", SearchTerms: []string{"environmenttransportationpublicsafetystandingcommittee", "environmenttransportationpublicsafetystandingcommitteesittingasthetransitwindsorboardofdirectors"}}
	CSSC    = MeetingType{Code: "CSSC", Name: "Community Services Standing Committee", SearchTerms: []string{"communityservicesstandingcommittee"}}
	Other   = MeetingType{Code: "Other", Name: "Other", SearchTerms: []string{"other", "inauguralmeetingofcitycouncil", "accountabilitytransparencyorientation"}}
)

func (mt MeetingType) hasString(s string) bool {
	return s == mt.Code || s == mt.Name || slices.Contains(mt.SearchTerms, s)
}

var searchRe = regexp.MustCompile(`[^a-zA-z0-9]+`)

func normalizeMeetingName(meetingName string) string {
	return strings.ToLower(searchRe.ReplaceAllString(meetingName, ""))
}

// GetMeetingType will return the matching MeetingType if the meeting argument matches the Code, Name or any of the search terms.
func GetMeetingType(meeting string) MeetingType {
	str := normalizeMeetingName(meeting)
	if CC.hasString(str) {
		return CC
	}
	if DHSC.hasString(str) {
		return DHSC
	}
	if Special.hasString(str) {
		return Special
	}
	if ETP.hasString(str) {
		return ETP
	}
	if CSSC.hasString(str) {
		return CSSC
	}
	if Other.hasString(str) {
		return MeetingType{Code: "Other", Name: meeting}
	}
	return MeetingType{Code: "Unknown", Name: str}
}

// Document represents the metadata associated with a given upstream document.
type Document struct {
	Link     string      `json:"link"`
	Name     string      `json:"name"`
	Meeting  MeetingType `json:"meeting"`
	Date     time.Time   `json:"date"`
	RawTitle string      `json:"rawTitle"`
}

// FilterFunc is a function type that returns a subset of the input documents.
type FilterFunc func([]Document) []Document

// Before returns a FilterFunc for documents before a given time.Time
func Before(t time.Time) FilterFunc {
	return func(docs []Document) []Document {
		out := make([]Document, 0)
		for _, doc := range docs {
			if doc.Date.Before(t) {
				out = append(out, doc)
			}
		}
		return out
	}
}

// After returns a FilterFunc for documents after a given time.Time
func After(t time.Time) FilterFunc {
	return func(docs []Document) []Document {
		out := make([]Document, 0)
		for _, doc := range docs {
			if doc.Date.After(t) {
				out = append(out, doc)
			}
		}
		return out
	}
}

// ByYear returns a FilterFunc for documents that match a given year
func ByYear(year int) FilterFunc {
	return func(docs []Document) []Document {
		out := make([]Document, 0)
		for _, doc := range docs {
			if doc.Date.Year() == year {
				out = append(out, doc)
			}
		}
		return out
	}
}

// ByMeetingType returns a FilterFunc for documents that match a specific MeetingType
func ByMeetingType(meeting MeetingType) FilterFunc {
	return func(docs []Document) []Document {
		out := make([]Document, 0)
		for _, doc := range docs {
			if doc.Meeting.Code == meeting.Code {
				out = append(out, doc)
			}
		}
		return out
	}
}

// ByStringInName returns a FilterFunc for a slice of Document that have a given string in its name
func ByStringInName(str string) FilterFunc {
	return func(docs []Document) []Document {
		out := make([]Document, 0)
		for _, doc := range docs {
			if str != "" {
				if strings.Contains(doc.Name, str) {
					out = append(out, doc)
				}
			} else {
				out = append(out, doc)
			}
		}
		return out
	}
}

// GetDocuments fetches and parses data from the upstream and returns a slice of Document. It currently only supports PDFs.
func GetDocuments(ctx context.Context) ([]Document, error) {
	cards, err := getHtmlCards(ctx, http.DefaultClient)
	if err != nil {
		return nil, err
	}

	return getDocumentFromCards(cards)
}

// getDocumentFromCards returns a slice of Document from a slice of htmlCard.
func getDocumentFromCards(cards []htmlCard) ([]Document, error) {
	docs := make([]Document, 0)
	for _, card := range cards {
		for _, link := range card.Links {
			title, err := url.PathUnescape(card.Title)
			if err != nil {
				title = card.Title
			}

			if path.Ext(link) == ".pdf" {
				doc, err := parseDocument(link, title)
				if err != nil {
					return nil, err
				}
				docs = append(docs, doc)
			}
		}
	}
	return docs, nil
}

var (
	dateLayout = "Monday, January 2, 2006"
	dateRegex  = regexp.MustCompile(`\b(?:Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday),\s+(January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},\s+\d{4}\b`)
)

// parseDocument returns a Document from a given htmlCard link and title
func parseDocument(link string, title string) (Document, error) {
	linkName := path.Base(link)
	name, err := url.PathUnescape(linkName)
	if err != nil {
		name = linkName
	}

	meetingDate := strings.Split(title, " - ")
	dateStr := dateRegex.FindString(title)
	if dateStr == "" {
		return Document{}, fmt.Errorf("scraper: could not find meeting date in title %q", title)
	}
	date, err := time.Parse(dateLayout, dateStr)
	if err != nil {
		return Document{}, fmt.Errorf("scraper: parse meeting date %q: %w", dateStr, err)
	}

	meetingName := title
	if len(meetingDate) > 0 {
		meetingName = meetingDate[0]
	}
	meeting := GetMeetingType(meetingName)
	return Document{
		Link:     link,
		Meeting:  meeting,
		Name:     name,
		Date:     date,
		RawTitle: title,
	}, nil
}

// getHtmlCards performs a GET request to the upstream and returns a slice of htmlCard.
func getHtmlCards(ctx context.Context, client *http.Client) ([]htmlCard, error) {
	const meetingUrl = "https://opendata.citywindsor.ca/Tools/CouncilAgendas?returnUrl=https://citywindsor.ca/cityhall/City-Council-Meetings/Pages/default.aspx"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, meetingUrl, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scraper: unexpected status code %s", resp.Status)
	}

	n, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}
	return cardsFromNodeRecursive(n), nil
}

// htmlCard is a html.Node with an extracted title and slice of links
type htmlCard struct {
	Title string
	Links []string
	*html.Node
}

// cardsFromNodeRecursive recursively checks the html.Node elements and converts valid elements into a slice of htmlCard.
func cardsFromNodeRecursive(node *html.Node) []htmlCard {
	cards := make([]htmlCard, 0)
	var fetchFn func(*html.Node)
	fetchFn = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, attr := range n.Attr {
				if strings.Contains(attr.Val, "CA_CouncilAgenda") {
					cards = append(cards, htmlCard{
						Title: titleFromNodeRecursive(n),
						Links: linksFromNodeRecursive(n),
						Node:  n,
					})
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			fetchFn(c)
		}
	}
	fetchFn(node)
	return cards
}

// titleFromNodeRecursive recursively checks the html.Node elements and returns the htmlCard title.
func titleFromNodeRecursive(node *html.Node) string {
	var title string
	var fetchFn func(*html.Node)
	fetchFn = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "strong" {
			title = n.FirstChild.Data
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			fetchFn(c)
		}
	}
	fetchFn(node)
	return title
}

// linksFromNodeRecursive recursively checks the html.Node elements and returns the htmlCard slice of link.
func linksFromNodeRecursive(node *html.Node) []string {
	links := make([]string, 0)
	var fetchFn func(*html.Node)
	fetchFn = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					links = append(links, attr.Val)
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			fetchFn(c)
		}
	}
	fetchFn(node)
	return links
}
