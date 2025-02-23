package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/goccy/go-yaml"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	rhtml "github.com/yuin/goldmark/renderer/html"
)

const defaultBaseUrl = "https://ricoberger.de"
const defaultImage = "/assets/img/social-preview.png"

type Data struct {
	Metadata Metadata
	Content  any
}

type Metadata struct {
	Title       string
	Description string
	Author      string
	Keywords    []string
	BaseUrl     string
	Url         string
	Image       string
	Prism       bool
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

type BlogPost struct {
	ID          string
	Title       string
	Description string
	AuthorName  string
	AuthorTitle string
	AuthorImage string
	PublishedAt time.Time
	Tags        []string
	Image       string
	Content     template.HTML
}

type BlogTag struct {
	Tag   string
	Posts []BlogPost
}

type RssFeedXml struct {
	XMLName          xml.Name `xml:"rss"`
	Version          string   `xml:"version,attr"`
	ContentNamespace string   `xml:"xmlns:content,attr"`
	Channel          *RssFeed
}

type RssDescription struct {
	XMLName xml.Name `xml:"description"`
	Content string   `xml:",cdata"`
}

type RssImage struct {
	XMLName xml.Name `xml:"image"`
	Url     string   `xml:"url"`
	Title   string   `xml:"title"`
	Link    string   `xml:"link"`
	Width   int      `xml:"width,omitempty"`
	Height  int      `xml:"height,omitempty"`
}

type RssFeed struct {
	XMLName       xml.Name `xml:"channel"`
	Title         string   `xml:"title"`
	Link          string   `xml:"link"`
	Description   string   `xml:"description"`
	Language      string   `xml:"language,omitempty"`
	Copyright     string   `xml:"copyright,omitempty"`
	PubDate       string   `xml:"pubDate,omitempty"`
	LastBuildDate string   `xml:"lastBuildDate,omitempty"`
	Image         *RssImage
	Items         []*RssItem `xml:"item"`
}

type RssItem struct {
	XMLName     xml.Name `xml:"item"`
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description *RssDescription
	Author      string `xml:"author,omitempty"`
	Enclosure   *RssEnclosure
	Guid        *RssGuid
	PubDate     string `xml:"pubDate,omitempty"`
}

type RssEnclosure struct {
	XMLName xml.Name `xml:"enclosure"`
	Url     string   `xml:"url,attr"`
	Type    string   `xml:"type,attr"`
}

type RssGuid struct {
	XMLName     xml.Name `xml:"guid"`
	Id          string   `xml:",chardata"`
	IsPermaLink string   `xml:"isPermaLink,attr,omitempty"`
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

	slog.Info("Build page not found...")
	err = buildPageNotFound()
	if err != nil {
		slog.Error("Failed to build page not found", slog.Any("error", err))
	}

	slog.Info("Build cheat sheets...")
	err = buildCheatSheets()
	if err != nil {
		slog.Error("Failed to build cheat sheets", slog.Any("error", err))
	}

	slog.Info("Build blog...")
	err = buildBlog()
	if err != nil {
		slog.Error("Failed to build blog", slog.Any("error", err))
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
					rhtml.WithUnsafe(),
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
			Title:       "Home - Rico Berger",
			Description: "Site Reliability Engineer, Hacker, Cloud Native Enthusiast",
			Author:      "Rico Berger",
			Keywords:    []string{"Rico Berger", "Site Reliability Engineer", "Hacker", "Cloud Native Enthusiast"},
			BaseUrl:     defaultBaseUrl,
			Url:         "/",
			Image:       defaultImage,
			Prism:       false,
		},
	}

	if err := buildTemplate("home", "./dist", homeData); err != nil {
		return err
	}

	var aboutData = Data{
		Metadata: Metadata{
			Title:       "About - Rico Berger",
			Description: "Site Reliability Engineer, Hacker, Cloud Native Enthusiast",
			Author:      "Rico Berger",
			Keywords:    []string{"Rico Berger", "Site Reliability Engineer", "Hacker", "Cloud Native Enthusiast"},
			BaseUrl:     defaultBaseUrl,
			Url:         "/about/",
			Image:       defaultImage,
			Prism:       false,
		},
	}

	if err := buildTemplate("about", "./dist/about", aboutData); err != nil {
		return err
	}

	if err := os.CopyFS("./dist/assets", os.DirFS("./templates/assets")); err != nil {
		return err
	}

	return nil
}

func buildPageNotFound() error {
	var pageNotFoundData = Data{
		Metadata: Metadata{
			Title:       "404 - Not Found - Rico Berger",
			Description: "Site Reliability Engineer, Hacker, Cloud Native Enthusiast",
			Author:      "Rico Berger",
			Keywords:    []string{"Rico Berger", "Site Reliability Engineer", "Hacker", "Cloud Native Enthusiast"},
			BaseUrl:     defaultBaseUrl,
			Url:         "/",
			Image:       defaultImage,
			Prism:       false,
		},
	}

	templates, err := template.New("base.html").ParseFiles("templates/base.html", "templates/404.html")
	if err != nil {
		return err
	}

	f, err := os.Create("./dist/404.html")
	if err != nil {
		return err
	}

	if err := templates.Execute(f, pageNotFoundData); err != nil {
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
			Title:       "Cheat Sheets - Rico Berger",
			Description: "Cheat Sheets about Site Reliability Engineering, Platform Engineering, Cloud Native, Kubernetes and more",
			Author:      "Rico Berger",
			Keywords:    []string{"Rico Berger", "Cheat Sheets"},
			BaseUrl:     defaultBaseUrl,
			Url:         "/cheat-sheets/",
			Image:       defaultImage,
			Prism:       false,
		},
		Content: cheatSheets,
	}

	if err := buildTemplate("cheat-sheets", "./dist/cheat-sheets", cheatsheetsData); err != nil {
		return err
	}

	for _, cheatSheet := range cheatSheets {
		if err := buildTemplate("cheat-sheet", fmt.Sprintf("./dist/cheat-sheets/%s", cheatSheet.ID), Data{
			Metadata: Metadata{
				Title:       fmt.Sprintf("%s - Cheat Sheet - Rico Berger", cheatSheet.Title),
				Description: cheatSheet.Description,
				Author:      cheatSheet.Author,
				Keywords:    cheatSheet.Keywords,
				BaseUrl:     defaultBaseUrl,
				Url:         fmt.Sprintf("/cheat-sheets/%s/", cheatSheet.ID),
				Image:       fmt.Sprintf("/cheat-sheets/%s/assets/%s-cheat-sheet.png", cheatSheet.ID, cheatSheet.ID),
				Prism:       true,
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

func buildBlog() error {
	files, err := os.ReadDir("./blog")
	if err != nil {
		return err
	}

	var posts []BlogPost

	for _, file := range files {
		if file.IsDir() {
			content, err := os.ReadFile(fmt.Sprintf("./blog/%s/%s.md", file.Name(), file.Name()))
			if err != nil {
				return err
			}

			var buf bytes.Buffer
			markdown := goldmark.New(
				goldmark.WithExtensions(
					meta.Meta,
					extension.Table,
					extension.Strikethrough,
					extension.Footnote,
				),
				goldmark.WithRendererOptions(
					rhtml.WithUnsafe(),
				),
			)
			context := parser.NewContext()

			if err := markdown.Convert(content, &buf, parser.WithContext(context)); err != nil {
				return err
			}

			metaData := meta.Get(context)

			publishedAt, err := time.Parse("2006-01-02 15:04:05", metaData["PublishedAt"].(string))
			if err != nil {
				return err
			}

			var tags []string
			for _, tag := range metaData["Tags"].([]any) {
				tags = append(tags, tag.(string))
			}

			image := defaultImage
			if val, ok := metaData["Image"]; ok && val != nil {
				image = val.(string)
			}

			post := BlogPost{
				ID:          file.Name(),
				Title:       metaData["Title"].(string),
				Description: metaData["Description"].(string),
				AuthorName:  metaData["AuthorName"].(string),
				AuthorTitle: metaData["AuthorTitle"].(string),
				AuthorImage: metaData["AuthorImage"].(string),
				PublishedAt: publishedAt,
				Tags:        tags,
				Image:       image,
				Content:     template.HTML(buf.String()),
			}

			posts = append(posts, post)
		}
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].PublishedAt.After(posts[j].PublishedAt)
	})

	var blogData = Data{
		Metadata: Metadata{
			Title:       "Blog - Rico Berger",
			Description: "Personal Blog about Site Reliability Engineering, Platform Engineering, Cloud Native, Kubernetes and more",
			Author:      "Rico Berger",
			Keywords:    []string{"Rico Berger", "Blog"},
			BaseUrl:     defaultBaseUrl,
			Url:         "/blog/",
			Image:       defaultImage,
			Prism:       false,
		},
		Content: posts,
	}

	if err := buildTemplate("blog", "./dist/blog", blogData); err != nil {
		return err
	}

	if err := buildRssFeed("./dist/blog", blogData.Metadata, posts); err != nil {
		return err
	}

	tags := make(map[string][]BlogPost)

	for _, post := range posts {
		if err := buildTemplate("blog-post", fmt.Sprintf("./dist/blog/posts/%s", post.ID), Data{
			Metadata: Metadata{
				Title:       fmt.Sprintf("%s - Blog - Rico Berger", post.Title),
				Description: post.Description,
				Author:      post.AuthorName,
				Keywords:    post.Tags,
				BaseUrl:     defaultBaseUrl,
				Url:         fmt.Sprintf("/blog/%s/", post.ID),
				Image:       post.Image,
				Prism:       true,
			},
			Content: post,
		}); err != nil {
			return err
		}

		hasAssets, err := exists(fmt.Sprintf("./blog/%s/assets", post.ID))
		if err != nil {
			return err
		}
		if hasAssets {
			if err := os.CopyFS(fmt.Sprintf("./dist/blog/posts/%s/assets", post.ID), os.DirFS(fmt.Sprintf("./blog/%s/assets", post.ID))); err != nil {
				return err
			}
		}

		for _, tag := range post.Tags {
			if tagPosts, ok := tags[tag]; ok {
				tags[tag] = append(tagPosts, post)
			} else {
				tags[tag] = []BlogPost{post}
			}
		}
	}

	for key, val := range tags {
		tagData := Data{
			Metadata: Metadata{
				Title:       fmt.Sprintf("%s - Blog - Rico Berger", key),
				Description: fmt.Sprintf("Blog Posts about %s", key),
				Author:      "Rico Berger",
				Keywords:    []string{"Rico Berger", "Blog", key},
				BaseUrl:     defaultBaseUrl,
				Url:         fmt.Sprintf("/blog/tags/%s/", key),
				Image:       defaultImage,
				Prism:       false,
			},
			Content: BlogTag{
				Tag:   key,
				Posts: val,
			},
		}

		if err := buildTemplate("blog-tag", fmt.Sprintf("./dist/blog/tags/%s", key), tagData); err != nil {
			return err
		}

		if err := buildRssFeed(fmt.Sprintf("./dist/blog/tags/%s", key), tagData.Metadata, val); err != nil {
			return err
		}
	}

	return nil
}

func buildRssFeed(distPath string, metadata Metadata, posts []BlogPost) error {
	var rssItems []*RssItem

	for _, post := range posts {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(post.Content)))
		if err != nil {
			return err
		}

		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			if href, ok := s.Attr("href"); ok {
				if strings.HasPrefix(href, "./") || strings.HasPrefix(href, "/") {
					base, err := url.Parse(fmt.Sprintf("%s/blog/posts/%s/", defaultBaseUrl, post.ID))
					if err != nil {
						return
					}
					rel, err := url.Parse(href)
					if err != nil {
						return
					}
					s.SetAttr("href", base.ResolveReference(rel).String())
				}
			}
		})

		doc.Find("img").Each(func(i int, s *goquery.Selection) {
			if src, ok := s.Attr("src"); ok {
				if strings.HasPrefix(src, "./") || strings.HasPrefix(src, "/") {
					base, err := url.Parse(fmt.Sprintf("%s/blog/posts/%s/", defaultBaseUrl, post.ID))
					if err != nil {
						return
					}
					rel, err := url.Parse(src)
					if err != nil {
						return
					}
					s.SetAttr("src", base.ResolveReference(rel).String())
				}
			}
		})

		content, err := doc.Find("body").Html()
		if err != nil {
			return err
		}

		rssItems = append(rssItems, &RssItem{
			Title: post.Title,
			Link:  fmt.Sprintf("%s/blog/posts/%s/", defaultBaseUrl, post.ID),
			Description: &RssDescription{
				Content: content,
			},
			Author: post.AuthorName,
			Enclosure: &RssEnclosure{
				Url:  fmt.Sprintf("%s%s", defaultBaseUrl, post.Image),
				Type: mime.TypeByExtension(filepath.Ext(post.Image)),
			},
			Guid: &RssGuid{
				Id:          fmt.Sprintf("%s/blog/posts/%s/", defaultBaseUrl, post.ID),
				IsPermaLink: "true",
			},
			PubDate: post.PublishedAt.Format(time.RFC1123Z),
		})
	}

	rssFeed := RssFeedXml{
		Version: "2.0",
		Channel: &RssFeed{
			Title:         metadata.Title,
			Link:          fmt.Sprintf("%s%s", defaultBaseUrl, metadata.Url),
			Description:   metadata.Description,
			Language:      "en-us",
			Copyright:     "Rico Berger",
			PubDate:       time.Now().Format(time.RFC1123Z),
			LastBuildDate: time.Now().Format(time.RFC1123Z),
			Image: &RssImage{
				Url:    fmt.Sprintf("%s/assets/img/icons/icon.png", defaultBaseUrl),
				Title:  "Rico Berger",
				Link:   defaultBaseUrl,
				Width:  1024,
				Height: 1024,
			},
			Items: rssItems,
		},
		ContentNamespace: "http://purl.org/rss/1.0/modules/content/",
	}

	data, err := xml.Marshal(rssFeed)
	if err != nil {
		return err
	}

	err = os.WriteFile(fmt.Sprintf("%s/feed.xml", distPath), data, 0666)
	if err != nil {
		return err
	}

	return nil
}
