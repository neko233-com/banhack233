// Command docs builds an offline-capable documentation site from release sources.
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

//go:embed assets/*
var assets embed.FS

const repository = "https://github.com/neko233-com/banhack233"

type page struct{ Source, File, Title, Language, Alternate string }

var pages = []page{
	{"README.md", "index.html", "使用指南", "zh-CN", "en.html"},
	{"README-EN.md", "en.html", "User guide", "en", "index.html"},
	{"docs/commands.md", "commands.html", "命令参考", "zh-CN", "commands-en.html"},
	{"docs/commands-en.md", "commands-en.html", "Commands", "en", "commands.html"},
	{"docs/configuration.md", "configuration.html", "配置参考", "zh-CN", "configuration-en.html"},
	{"docs/configuration-en.md", "configuration-en.html", "Configuration", "en", "configuration.html"},
	{"docs/operations.md", "operations.html", "部署与恢复", "zh-CN", "operations-en.html"},
	{"docs/operations-en.md", "operations-en.html", "Operations", "en", "operations.html"},
	{"docs/troubleshooting.md", "troubleshooting.html", "排障与 FAQ", "zh-CN", "troubleshooting-en.html"},
	{"docs/troubleshooting-en.md", "troubleshooting-en.html", "Troubleshooting", "en", "troubleshooting.html"},
	{"docs/design.md", "design.html", "原理与边界", "zh-CN", "design-en.html"},
	{"docs/design-en.md", "design-en.html", "Architecture & limits", "en", "design.html"},
	{"docs/notifications.md", "notifications.html", "通知渠道", "zh-CN", "notifications-en.html"},
	{"docs/notifications-en.md", "notifications-en.html", "Notifications", "en", "notifications.html"},
	{"docs/updates.md", "updates.html", "自动更新", "zh-CN", "updates-en.html"},
	{"docs/updates-en.md", "updates-en.html", "Automatic updates", "en", "updates.html"},
	{"docs/releasing.md", "releasing.html", "开发与发布", "zh-CN", "releasing-en.html"},
	{"docs/releasing-en.md", "releasing-en.html", "Development & releases", "en", "releasing.html"},
	{"CHANGELOG.md", "changelog.html", "版本记录 / Changelog", "zh-CN", "en.html"},
}

type heading struct {
	ID, Title string
	Level     int
}

type documentLink struct {
	File, Title string
	Current     bool
}

type metadata struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuiltAt    string `json:"built_at"`
	Repository string `json:"repository"`
}
type view struct {
	page
	metadata
	Content                                                                                                       template.HTML
	TOC                                                                                                           []heading
	Documents                                                                                                     []documentLink
	Guide, Releases, Navigation, Search, Empty, Menu, Copy, Copied, CopyFailed, SourceLabel, ReleaseLabel, Footer string
	SourceURL, ReleaseURL                                                                                         string
}

func main() {
	root := flag.String("root", "../..", "repository root")
	out := flag.String("out", "../../site", "output directory")
	version := flag.String("version", "dev", "release tag")
	commit := flag.String("commit", "local", "source commit")
	built := flag.String("date", time.Now().UTC().Format(time.RFC3339), "build timestamp")
	serve := flag.String("serve", "", "optional local preview address, e.g. 127.0.0.1:8088")
	flag.Parse()
	meta := metadata{*version, *commit, *built, repository}
	if err := build(*root, *out, meta); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Built %d documentation pages in %s (%s)\n", len(pages), *out, *version)
	if *serve != "" {
		fmt.Printf("Preview: http://%s\n", *serve)
		if err := http.ListenAndServe(*serve, http.FileServer(http.Dir(*out))); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func build(root, out string, meta metadata) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	if root == out {
		return fmt.Errorf("output must not be the repository root")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	tmpl, err := template.ParseFS(assets, "assets/page.html")
	if err != nil {
		return err
	}
	for _, p := range pages {
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p.Source)))
		if err != nil {
			return err
		}
		body, toc, err := render(source, p.Source, meta.Version)
		if err != nil {
			return err
		}
		v := view{page: p, metadata: meta, Content: template.HTML(body), TOC: toc,
			Guide: "index.html", Releases: "releasing.html", Navigation: "文档导航", Search: "查找章节…", Empty: "没有匹配的章节", Menu: "目录", Copy: "复制", Copied: "已复制", CopyFailed: "请手动选择并复制", SourceLabel: "查看源码", ReleaseLabel: "下载此版本", Footer: "fail2ban / sshguard 风格 · 系统巡检 · 保活 · 通知 · 自启动",
			SourceURL: repository + "/blob/" + url.PathEscape(meta.Version) + "/" + p.Source, ReleaseURL: repository + "/releases/tag/" + url.PathEscape(meta.Version)}
		if p.Language == "en" {
			v.Guide = "en.html"
			v.Releases = "releasing-en.html"
			v.Navigation = "On this page"
			v.Search = "Find a section…"
			v.Empty = "No matching sections"
			v.Menu = "Contents"
			v.Copy = "Copy"
			v.Copied = "Copied"
			v.CopyFailed = "Select and copy manually"
			v.SourceLabel = "View source"
			v.ReleaseLabel = "Download release"
			v.Footer = "fail2ban / sshguard style · Host audits · Keepalive · Notifications · Autostart"
		}
		for _, topic := range pages {
			if topic.Language == p.Language || topic.File == "changelog.html" {
				v.Documents = append(v.Documents, documentLink{topic.File, topic.Title, topic.File == p.File})
			}
		}
		if meta.Version == "dev" {
			v.SourceURL = repository + "/blob/main/" + p.Source
			v.ReleaseURL = repository + "/releases/latest"
		}
		var output bytes.Buffer
		if err := tmpl.Execute(&output, v); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, p.File), output.Bytes(), 0o644); err != nil {
			return err
		}
	}
	for _, name := range []string{"docs.css", "docs.js"} {
		b, err := assets.ReadFile("assets/" + name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, name), b, 0o644); err != nil {
			return err
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "configs", "config.json.example"))
	if err != nil {
		return err
	}
	if !json.Valid(b) {
		return fmt.Errorf("invalid example config JSON")
	}
	if err := os.WriteFile(filepath.Join(out, "config.json.example"), b, 0o644); err != nil {
		return err
	}
	b, err = json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "version.json"), append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, ".nojekyll"), nil, 0o644)
}

func render(source []byte, sourcePath, version string) (string, []heading, error) {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := md.Parser().Parse(text.NewReader(source))
	var toc []heading
	ids := map[string]int{}
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Heading:
			title := string(n.Text(source))
			slug := headingID(title)
			ids[slug]++
			if ids[slug] > 1 {
				slug = fmt.Sprintf("%s-%d", slug, ids[slug])
			}
			n.SetAttributeString("id", []byte(slug))
			if n.Level == 2 || n.Level == 3 {
				toc = append(toc, heading{slug, title, n.Level})
			}
		case *ast.Link:
			dest, err := rewriteLink(string(n.Destination), sourcePath, version)
			if err != nil {
				return ast.WalkStop, err
			}
			n.Destination = []byte(dest)
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return "", nil, err
	}
	var body bytes.Buffer
	if err := md.Renderer().Render(&body, source, doc); err != nil {
		return "", nil, err
	}
	return body.String(), toc, nil
}

func headingID(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "section"
	}
	return s
}

func rewriteLink(dest, source, version string) (string, error) {
	if version == "dev" {
		version = "main"
	}
	u, err := url.Parse(dest)
	if err != nil {
		return "", err
	}
	if u.IsAbs() || u.Host != "" || u.Path == "" {
		return dest, nil
	}
	p := path.Clean(path.Join(path.Dir(source), u.Path))
	for _, page := range pages {
		if page.Source == p {
			u.Path = page.File
			return u.String(), nil
		}
	}
	if p == "configs/config.json.example" {
		u.Path = "config.json.example"
		return u.String(), nil
	}
	if strings.HasSuffix(p, ".md") {
		return "", fmt.Errorf("unmapped documentation link %q in %s", dest, source)
	}
	return repository + "/blob/" + url.PathEscape(version) + "/" + p, nil
}
