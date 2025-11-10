package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dntiontk/civic-code/pkg/scraper"
)

func TestDownloadDocuments_Success(t *testing.T) {
	const remoteFileName = "agenda.pdf"
	const fileBody = "sample-pdf-content"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+remoteFileName {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(fileBody))
	}))
	t.Cleanup(srv.Close)

	origClient := httpClient
	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = origClient })

	destDir := t.TempDir()
	ctx := context.Background()

	doc := scraper.Document{
		Link:    srv.URL + "/" + remoteFileName,
		Name:    remoteFileName,
		Meeting: scraper.MeetingType{Code: "CC"},
		Date:    time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC),
	}
	doc.ApplyFileNameSchema()

	docs := []scraper.Document{doc}

	updated, err := DownloadDocuments(ctx, docs, destDir, 2)
	if err != nil {
		t.Fatalf("DownloadDocuments returned error: %v", err)
	}

	if len(updated) != 1 {
		t.Fatalf("expected 1 document, got %d", len(updated))
	}

	gotChecksum := updated[0].Checksum
	wantChecksum := sha256.Sum256([]byte(fileBody))
	if gotChecksum != hex.EncodeToString(wantChecksum[:]) {
		t.Fatalf("unexpected checksum: got %q want %q", gotChecksum, hex.EncodeToString(wantChecksum[:]))
	}

	if updated[0].FileName != doc.FileName {
		t.Fatalf("unexpected file name: got %q want %q", updated[0].FileName, doc.FileName)
	}

	filePath := filepath.Join(destDir, doc.FileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(data) != fileBody {
		t.Fatalf("unexpected file contents: got %q want %q", string(data), fileBody)
	}
}

func TestDownloadDocuments_ExistingChecksumSkipsDownload(t *testing.T) {
	destDir := t.TempDir()
	const remoteFileName = "existing.pdf"
	fileContents := []byte("cached-content")

	origClient := httpClient
	httpClient = &http.Client{
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("unexpected network call")
		}),
	}
	t.Cleanup(func() { httpClient = origClient })

	ctx := context.Background()
	doc := scraper.Document{
		Link:    "https://example.invalid/" + remoteFileName,
		Name:    remoteFileName,
		Meeting: scraper.MeetingType{Code: "CC"},
		Date:    time.Date(2024, time.February, 10, 0, 0, 0, 0, time.UTC),
	}
	doc.ApplyFileNameSchema()

	existingPath := filepath.Join(destDir, doc.FileName)
	if err := os.WriteFile(existingPath, fileContents, 0o644); err != nil {
		t.Fatalf("write cached file: %v", err)
	}
	sum := sha256.Sum256(fileContents)
	expectedChecksum := hex.EncodeToString(sum[:])

	doc.Checksum = expectedChecksum

	docs := []scraper.Document{
		doc,
	}

	updated, err := DownloadDocuments(ctx, docs, destDir, 1)
	if err != nil {
		t.Fatalf("DownloadDocuments returned error: %v", err)
	}

	if updated[0].Checksum != expectedChecksum {
		t.Fatalf("checksum changed unexpectedly: got %q want %q", updated[0].Checksum, expectedChecksum)
	}
}

func TestDownloadDocuments_ChecksumMismatch(t *testing.T) {
	const remoteFileName = "mismatch.pdf"
	const fileBody = "remote-content"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fileBody))
	}))
	t.Cleanup(srv.Close)

	origClient := httpClient
	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = origClient })

	ctx := context.Background()
	destDir := t.TempDir()

	doc := scraper.Document{
		Link:     srv.URL + "/" + remoteFileName,
		Name:     remoteFileName,
		Meeting:  scraper.MeetingType{Code: "CC"},
		Date:     time.Date(2024, time.January, 20, 0, 0, 0, 0, time.UTC),
		Checksum: "deadbeef",
	}
	doc.ApplyFileNameSchema()

	docs := []scraper.Document{
		doc,
	}

	updated, err := DownloadDocuments(ctx, docs, destDir, 2)
	if err == nil {
		t.Fatalf("expected checksum mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}

	if len(updated) != 1 {
		t.Fatalf("expected 1 updated document, got %d", len(updated))
	}

	// The file should not be left behind on mismatch.
	if _, statErr := os.Stat(filepath.Join(destDir, doc.FileName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no file on checksum mismatch, got err=%v", statErr)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
