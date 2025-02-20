package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

type Data struct {
	Metadata Metadata
	Content  any
}

type Metadata struct {
	Title       string
	Description string
	Author      string
	Keywords    []string
}

type CheatSheet struct {
	ID          string           `yaml:"id"`
	Title       string           `yaml:"title"`
	Description string           `yaml:"description"`
	Author      string           `yaml:"author"`
	Keywords    []string         `yaml:"keywords"`
	Pages       []CheatSheetPage `yaml:"pages"`
}

type CheatSheetPage struct {
	Title    string              `yaml:"title"`
	Columns  int64               `yaml:"columns"`
	Sections []CheatSheetSection `yaml:"sections"`
}

type CheatSheetSection struct {
	Title string        `yaml:"title"`
	Items []string      `yaml:"items"`
	Tip   CheatSheetTip `yaml:"tip"`
}

type CheatSheetTip struct {
	Description string   `yaml:"description"`
	Items       []string `yaml:"items"`
}

func main() {
	var serve bool

	flag.BoolVar(&serve, "serve", false, "Start a local server to preview the generated site.")
	flag.Parse()

	if err := build(); err != nil {
		slog.Error("Failed to build site", slog.Any("error", err))
		os.Exit(1)
	}

	if serve {
		fs := http.FileServer(http.Dir("./dist"))
		http.Handle("/", fs)

		slog.Info("Start server on :9999...")
		if err := http.ListenAndServe(":9999", nil); err != nil {
			slog.Error("Failed to start server", slog.Any("error", err))
			os.Exit(1)
		}
	}
}

func build() error {
	slog.Info("Start build...")

	slog.Info("Build home...")
	err := buildHome()
	if err != nil {
		slog.Error("Failed to build home", slog.Any("error", err))
	}

	slog.Info("Build cheat sheets...")
	err = buildCheatSheets()
	if err != nil {
		slog.Error("Failed to build cheat sheets", slog.Any("error", err))
	}

	slog.Info("Build done")
	return nil
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	return false, err
}

func buildTemplate(tmpl string, distPath string, data Data) error {
	if err := os.MkdirAll(distPath, os.ModePerm); err != nil {
		return err
	}

	templates, err := template.New("base.html").Funcs(template.FuncMap{
		"formatMarkdown": func(s string) template.HTML {
			md := goldmark.New(
				goldmark.WithExtensions(
					extension.Table,
					extension.Strikethrough,
				),
				goldmark.WithRendererOptions(
					html.WithUnsafe(),
				),
			)

			var buf bytes.Buffer
			if err := md.Convert([]byte(s), &buf); err != nil {
				slog.Error("Failed to convert markdown", slog.Any("error", err))
			}
			return template.HTML(buf.String())
		},
	}).ParseFiles("templates/base.html", fmt.Sprintf("templates/%s.html", tmpl))
	if err != nil {
		return err
	}

	f, err := os.Create(fmt.Sprintf("%s/index.html", distPath))
	if err != nil {
		return err
	}

	if err := templates.Execute(f, data); err != nil {
		return err
	}

	return nil
}

func buildHome() error {
	var homeData = Data{
		Metadata: Metadata{
			Title:       "Rico Berger - Site Reliability Engineer, Hacker, Cloud Native Enthusiast",
			Description: "Site Reliability Engineer, Hacker, Cloud Native Enthusiast",
			Author:      "Rico Berger",
			Keywords:    []string{"Rico Berger", "Site Reliability Engineer", "Hacker", "Cloud Native Enthusiast"},
		},
	}

	if err := buildTemplate("home", "./dist", homeData); err != nil {
		return err
	}

	if err := buildTemplate("about", "./dist/about", homeData); err != nil {
		return err
	}

	if err := os.CopyFS("./dist/assets", os.DirFS("./templates/assets")); err != nil {
		return err
	}

	return nil
}

func buildCheatSheets() error {
	files, err := os.ReadDir("./cheat-sheets")
	if err != nil {
		return err
	}

	var cheatSheets []CheatSheet

	for _, file := range files {
		if file.IsDir() {
			content, err := os.ReadFile(fmt.Sprintf("./cheat-sheets/%s/%s.yaml", file.Name(), file.Name()))
			if err != nil {
				return err
			}

			var cheatSheet CheatSheet
			if err := yaml.Unmarshal(content, &cheatSheet); err != nil {
				return err
			}
			cheatSheet.ID = file.Name()

			cheatSheets = append(cheatSheets, cheatSheet)
		}
	}

	var cheatsheetsData = Data{
		Metadata: Metadata{
			Title:       "Rico Berger - Cheat Sheets",
			Description: "Cheat Sheets",
			Author:      "Rico Berger",
			Keywords:    []string{"Rico Berger", "Cheat Sheets"},
		},
		Content: cheatSheets,
	}

	if err := buildTemplate("cheat-sheets", "./dist/cheat-sheets", cheatsheetsData); err != nil {
		return err
	}

	for _, cheatSheet := range cheatSheets {
		if err := buildTemplate("cheat-sheet", fmt.Sprintf("./dist/cheat-sheets/%s", cheatSheet.ID), Data{
			Metadata: Metadata{
				Title:       fmt.Sprintf("%s - Cheat Sheet", cheatSheet.Title),
				Description: cheatSheet.Description,
				Author:      cheatSheet.Author,
				Keywords:    cheatSheet.Keywords,
			},
			Content: cheatSheet,
		}); err != nil {
			return err
		}

		hasAssets, err := exists(fmt.Sprintf("./cheat-sheets/%s/assets", cheatSheet.ID))
		if err != nil {
			return err
		}
		if hasAssets {
			if err := os.CopyFS(fmt.Sprintf("./dist/cheat-sheets/%s/assets", cheatSheet.ID), os.DirFS(fmt.Sprintf("./cheat-sheets/%s/assets", cheatSheet.ID))); err != nil {
				return err
			}
		}
	}

	return nil
}
