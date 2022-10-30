package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gocolly/colly"
)

func main() {
	depth := flag.Int("depth", 1, "max traversal depth")
	flag.Parse()

	if len(flag.Args()) != 2 {
		fmt.Fprintln(os.Stderr, "Usage:", os.Args[0], "<URL> <OUTDIR>")
	}

	url := flag.Arg(0)
	outdir := flag.Arg(1)

	Scrape(*depth, url, outdir)
}

// Visits url and downloads all torrents to outdir to a certian depth
//
// Torrents with a name that already exists in outdir are skipped if
// the upstream file has the same file size as the one on disk
func Scrape(depth int, url, outdir string) {
	// create outdir if it doesn't exist
	err := os.MkdirAll(outdir, os.ModePerm)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Instantiate default collector
	c := colly.NewCollector(
		// MaxDepth is 1, so only the links on the scraped page
		// is visited, and no further links are followed
		colly.MaxDepth(depth + 1),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       time.Second,
	})

	// On every a element which has href attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")

		// Visit link found on page
		e.Request.Visit(link)
	})

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		pos := strings.LastIndex(r.URL.Path, ".")
		if pos != -1 && r.URL.Path[pos+1:len(r.URL.Path)] == "torrent" {
			// Check if we already have this file by name
			name := path.Base(r.URL.Path)
			file := outdir + "/" + name
			stat, err := os.Stat(file)

			if err != nil {
				if os.IsNotExist(err) {
					// Download
					download(r, file)
				} else {
					// Unrecoverable error
					fmt.Println(err)
				}
			} else {
				// Compare file size then Download
				fs_sz := stat.Size()

				fmt.Println("HEAD", r.URL)
				res, err := http.Get(r.URL.String())
				if err != nil {
					fmt.Println(err)
					return
				}

				if fs_sz != res.ContentLength {
					download(r, file)
				}
			}

		} else {
			fmt.Println("Visiting", r.URL.String())
		}
	})

	c.Visit(url)
}

// Downloads the file at `r` and saves it to `target` on disk
func download(r *colly.Request, target string) error {
	// Save this file to ourdir
	fmt.Println("GET", r.URL)

	res, err := http.Get(r.URL.String())
	if err != nil {
		return err
	}

	f, err := os.Create(target)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, res.Body)
	if err != nil {
		return err
	}

	return res.Body.Close()
}
