package scraper

import (
	"testing"
	"time"
)

func TestApplyFileNameSchema(t *testing.T) {
	doc := Document{
		Name: "City Council Agenda & Minutes.pdf",
		Meeting: MeetingType{
			Code: "CC",
		},
		Date: time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC),
	}

	doc.ApplyFileNameSchema()

	const want = "2024_03_15-CC-city_council_agenda_minutes.pdf"
	if doc.FileName != want {
		t.Fatalf("ApplyFileNameSchema => %q, want %q", doc.FileName, want)
	}
}

func TestApplyFileNameSchemaWithFallbacks(t *testing.T) {
	doc := Document{
		Name: "???",
		Meeting: MeetingType{
			Code: "",
		},
		Date: time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC),
	}

	doc.ApplyFileNameSchema()

	const want = "2024_01_02-UNKNOWN-document.pdf"
	if doc.FileName != want {
		t.Fatalf("ApplyFileNameSchema fallback => %q, want %q", doc.FileName, want)
	}
}
