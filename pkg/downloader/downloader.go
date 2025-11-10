package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/dntiontk/civic-code/pkg/scraper"
)

var httpClient = http.DefaultClient

type task struct {
	index int
	doc   scraper.Document
}

type result struct {
	index int
	doc   scraper.Document
	err   error
}

// DownloadDocuments downloads each document concurrently, computes a checksum and returns the updated slice.
func DownloadDocuments(ctx context.Context, docs []scraper.Document, destDir string, concurrency int) ([]scraper.Document, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	if concurrency < 1 {
		concurrency = 1
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("downloader: create destination dir: %w", err)
	}

	tasks := make(chan task)
	results := make(chan result, len(docs))

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				updated, err := downloadOne(ctx, t.doc, destDir)
				results <- result{
					index: t.index,
					doc:   updated,
					err:   err,
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	go func() {
		for idx, doc := range docs {
			select {
			case <-ctx.Done():
				results <- result{
					index: idx,
					doc:   doc,
					err:   ctx.Err(),
				}
			case tasks <- task{index: idx, doc: doc}:
			}
		}
		close(tasks)
	}()

	updated := make([]scraper.Document, len(docs))
	errs := make([]error, 0)

	for res := range results {
		if res.err != nil {
			errs = append(errs, fmt.Errorf("downloader: %s: %w", res.doc.Name, res.err))
		}
		updated[res.index] = res.doc
	}

	if len(errs) > 0 {
		return updated, errors.Join(errs...)
	}

	return updated, nil
}

func downloadOne(ctx context.Context, doc scraper.Document, destDir string) (scraper.Document, error) {
	fileName := doc.Name
	if fileName == "" {
		fileName = filepath.Base(doc.Link)
	}
	fileName = filepath.Base(fileName)
	if fileName == "" || fileName == "." {
		return doc, fmt.Errorf("missing file name")
	}

	destPath := filepath.Join(destDir, fileName)

	if doc.Checksum != "" {
		if sum, err := checksumForFile(destPath); err == nil {
			if sum == doc.Checksum {
				doc.Checksum = sum
				return doc, nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return doc, fmt.Errorf("read existing file checksum: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, doc.Link, nil)
	if err != nil {
		return doc, fmt.Errorf("create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return doc, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return doc, fmt.Errorf("download: unexpected status %s", resp.Status)
	}

	tempFile, err := os.CreateTemp(destDir, "download-*")
	if err != nil {
		return doc, fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name())
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tempFile, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return doc, fmt.Errorf("copy: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		return doc, fmt.Errorf("flush temp file: %w", err)
	}

	sum := hex.EncodeToString(hasher.Sum(nil))

	if doc.Checksum != "" && doc.Checksum != sum {
		return doc, fmt.Errorf("checksum mismatch (expected %s, got %s)", doc.Checksum, sum)
	}

	if err := tempFile.Close(); err != nil {
		return doc, fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tempFile.Name(), destPath); err != nil {
		return doc, fmt.Errorf("move temp file: %w", err)
	}

	doc.Checksum = sum
	return doc, nil
}

func checksumForFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
