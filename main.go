package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Version struct {
	Version string `json:"version,omitempty"`
}

type Data struct {
	Versions []Version `json:"Versions,omitempty"`
}

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	stdout, err := exec.Command("code", "--list-extensions", "--show-versions").Output()
	if err != nil {
		return err
	}

	exts := sort.StringSlice(strings.Split(strings.TrimSpace(string(stdout)), "\n"))
	for _, ext := range exts {
		id, current, _ := strings.Cut(ext, "@")
		publisher, name, _ := strings.Cut(id, ".")

		html, err := http.Get("https://marketplace.visualstudio.com/items?itemName=" + id)
		if err != nil {
			return err
		}

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

		latest := data.Versions[0].Version
		if current != latest {
			fmt.Printf(format, displayName, publisher, name, latest, current)
		}

		if err := html.Body.Close(); err != nil {
			return err
		}
	}

	return nil
}

const format = `{
  # %s
  publisher = "%s";
  name = "%s";
  version = "%s"; # From %q
  sha256 = "";
}
`
