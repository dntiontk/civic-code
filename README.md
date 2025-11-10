# civic-code

[![Go Reference](https://pkg.go.dev/badge/github.com/dntiontk/civic-code.svg)](https://pkg.go.dev/github.com/dntiontk/civic-code)

This repository is a suite of tools to enable the [civic code project](https://dntiontk.github.io/posts/civic-code/). 

## Tools

- [doc-search](doc-search/README.md)

### doc-search

`doc-search` is a tool for indexing [PDF documents hosted by the City of Windsor](https://opendata.citywindsor.ca/Tools/CouncilAgendas?returnUrl=https://citywindsor.ca/cityhall/City-Council-Meetings/Pages/default.aspx). It leverages the web-scraping developed during the [scraping council meetings](https://dntiontk.github.io/posts/scraping-council-meetings/) project.

#### Features

- Filter documents by year
- Filter documents by a specific date or date range
- Search documents based on meeting types
- Filter documents by name or keywords
- Download matching PDFs concurrently

#### Installation

The tool is prebuilt for the following platforms:

- Windows (amd64)
- Linux (amd64)
- macOS (arm64)

##### Download prebuilt binaries
You can download the prebuilt binaries from the [Releases](https://github.com/dntiontk/civic-code/releases) section.

1. Download the binary corresponding to your operating system.
2. Copy the binary to your `PATH`

##### Build from source

```bash
# 1. Clone the repository
git clone https://github.com/dntiontk/civic-code.git
cd civic-code/doc-search

# 2. Build the binary for your platform
GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) go build -o doc-search main.go

# 3. Copy the binary to your `PATH`
```

#### Usage

The tool downloads the matched PDFs and writes them to disk while emitting the document metadata (including checksums) as JSON. Available flags:

```
Usage of bin/doc-search:
  -after string
        filter documents after date
  -before string
        filter documents before date
  -docName string
        filter documents with string in name
  -concurrency int
        number of concurrent downloads (default 4)
  -downloadDir string
        directory to store downloaded PDFs (default "./downloads")
  -meetingType string
        filter documents by meeting type
  -year int
        filter documents by year (default -1)
```

## Contributing

Contributions are welcome. Please open an issue or submit a pull request for any enhancements or bug fixes.

