package main

import (
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
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

	slog.Info("Build done")
	return nil
}

func buildTemplate(tmpl string, distPath string, data Data) error {
	if err := os.MkdirAll(distPath, os.ModePerm); err != nil {
		return err
	}

	templates, err := template.ParseFiles("templates/base.html", fmt.Sprintf("templates/%s.html", tmpl))
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
