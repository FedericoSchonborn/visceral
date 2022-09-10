package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	// 1: ID
	PageFormat = "https://marketplace.visualstudio.com/items?itemName=%[1]s"
	// 1: Publisher
	// 2: Name
	// 3: Version
	DownloadFormat = "https://marketplace.visualstudio.com/_apis/public/gallery/publishers/%[1]s/vsextensions/%[2]s/%[3]s/vspackage"
	// 1: ID
	// 2: Version
	VSIXFormat = "%[1]s-%[2]s.vsix"
	// 1: Display Name
	// 2: Publisher
	// 3: Name
	// 4: Version
	// 5: SHA256
	DefaultFormat = `{
  # %[1]s
  publisher = "%[2]s";
  name = "%[3]s";
  version = "%[4]s";
  sha256 = "%[5]s";
}
`
	// 1: Display Name
	// 2: Publisher
	// 3: Name
	// 4: Latest
	// 5: Version
	// 6: SHA256
	UpdateFormat = `{
  # %[1]s
  publisher = "%[2]s";
  name = "%[3]s";
  version = "%[4]s"; # From %[5]q
  sha256 = "%[6]s";
}
`
)

type Version struct {
	Version string `json:"version,omitempty"`
}

type Data struct {
	Versions []Version `json:"Versions,omitempty"`
}

type Ext struct {
	ID        string
	Publisher string
	Name      string
	Version   string
}

func main() {
	if err := run(os.Args); err != nil {
		eprintfln("Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	stdout, err := exec.Command("code", "--list-extensions", "--show-versions").Output()
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	exts := make([]Ext, len(lines))
	for i, ext := range lines {
		id, version, _ := strings.Cut(ext, "@")
		publisher, name, _ := strings.Cut(id, ".")

		exts[i] = Ext{
			ID:        strings.ToLower(id),
			Publisher: strings.ToLower(publisher),
			Name:      strings.ToLower(name),
			Version:   version,
		}
	}
	sort.Slice(exts, func(i, j int) bool {
		return exts[i].Name < exts[j].Name
	})

	for _, ext := range exts {
		eprintfln("Fetching data for extension %s...", ext.ID)
		html, err := get(fmt.Sprintf(PageFormat, ext.ID))
		if err != nil {
			return err
		}
		defer html.Body.Close()

		doc, err := goquery.NewDocumentFromReader(html.Body)
		if err != nil {
			return err
		}

		displayName := doc.Find(".ux-item-name").First().Text()
		text := doc.Find(".rhs-content .jiContent").First().Text()

		var data Data
		if err := json.Unmarshal([]byte(text), &data); err != nil {
			return err
		}
		current := ext.Version
		latest := data.Versions[0].Version

		eprintfln("Downloading "+VSIXFormat+"...", ext.ID, latest)
		resp, err := get(fmt.Sprintf(DownloadFormat, ext.Publisher, ext.Name, latest))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		hash := sha256.New()
		if _, err := io.Copy(hash, resp.Body); err != nil {
			return err
		}
		sha256 := base64.StdEncoding.EncodeToString(hash.Sum(nil))

		if current == latest {
			fmt.Printf(DefaultFormat, displayName, ext.Publisher, ext.Name, latest, sha256)
		} else {
			fmt.Printf(UpdateFormat, displayName, ext.Publisher, ext.Name, latest, current, sha256)
		}
	}

	return nil
}

func get(url string) (*http.Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		secs, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		if err != nil {
			return nil, err
		}
		secs += 5 // Offset, just in case.

		eprintfln("Waiting for %d seconds, then retrying...", secs)
		time.Sleep(time.Duration(secs) * time.Second)
		return get(url)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	return resp, nil
}

func eprintfln(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}
