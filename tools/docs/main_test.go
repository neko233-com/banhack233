package main

import (
	"encoding/json"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRenderChineseAnchorsTablesAndLinks(t *testing.T) {
	source := []byte("# Guide\n\n## 安装与配置\n\n## 安装与配置\n\n| Key | Value |\n| --- | --- |\n| A | B |\n\n[English](README-EN.md) [Config](configs/config.json.example)\n\n```sh\necho '<HOST>'\n```\n")
	body, toc, err := render(source, "README.md", "v0.1.22")
	if err != nil {
		t.Fatal(err)
	}
	if len(toc) != 2 || toc[0].ID != "安装与配置" || toc[1].ID != "安装与配置-2" {
		t.Fatalf("toc=%+v", toc)
	}
	for _, want := range []string{"<table>", `href="en.html"`, `href="config.json.example"`, "&lt;HOST&gt;"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in HTML", want)
		}
	}
}

func TestRelativeLinksAndUnknownDocs(t *testing.T) {
	link, err := rewriteLink("../README.md#功能", "docs/releasing.md", "v0.1.22")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(link)
	if u.Path != "index.html" || u.Fragment != "功能" {
		t.Fatalf("link=%s", link)
	}
	if _, err := rewriteLink("missing.md", "README.md", "v0.1.22"); err == nil {
		t.Fatal("broken documentation link accepted")
	}
	link, err = rewriteLink("../internal/config/config.go", "docs/configuration.md", "dev")
	if err != nil || link != repository+"/blob/main/internal/config/config.go" {
		t.Fatalf("development source link=%s, err=%v", link, err)
	}
	body, _, err := render([]byte("<script>alert('unsafe')</script>"), "README.md", "v0.1.22")
	if err != nil || strings.Contains(body, "<script>") {
		t.Fatalf("raw HTML allowed: %s %v", body, err)
	}
}

func TestBuildRealDocsAndInternalLinks(t *testing.T) {
	out := t.TempDir()
	meta := metadata{"v0.1.22", "test-commit", "2026-10-06T00:00:00Z", repository}
	if err := build("../..", out, meta); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(out, "version.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got metadata
	if err := json.Unmarshal(manifest, &got); err != nil {
		t.Fatal(err)
	}
	if got != meta {
		t.Fatalf("metadata=%+v", got)
	}
	hrefs := regexp.MustCompile(`href="([^"]+)"`)
	for _, p := range pages {
		b, err := os.ReadFile(filepath.Join(out, p.File))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range hrefs.FindAllSubmatch(b, -1) {
			u, err := url.Parse(html.UnescapeString(string(match[1])))
			if err != nil {
				t.Fatal(err)
			}
			if u.IsAbs() || u.Host != "" {
				continue
			}
			file := u.Path
			if file == "" {
				file = p.File
			}
			target, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(file)))
			if err != nil {
				t.Errorf("%s link %s: %v", p.File, u, err)
				continue
			}
			if u.Fragment != "" && !strings.Contains(string(target), `id="`+html.EscapeString(u.Fragment)+`"`) {
				t.Errorf("missing anchor %s in %s", u.Fragment, file)
			}
		}
		if !strings.Contains(string(b), "test-commit") || !strings.Contains(string(b), "v0.1.22") {
			t.Errorf("missing version identity in %s", p.File)
		}
	}
}

func TestBuildPublishesOnlyDesignatedFiles(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	for _, p := range pages {
		target := filepath.Join(root, filepath.FromSlash(p.Source))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("# Documentation\n\n## Install\nhello"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "configs", "config.json.example"), []byte(`{"dry_run":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"password":"private-fixture"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := build(root, out, metadata{"dev", "local", "", repository}); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(pages)+5 {
		t.Fatalf("unexpected published files: %v", files)
	}
	if _, err := os.Stat(filepath.Join(out, "config.json")); !os.IsNotExist(err) {
		t.Fatal("local config entered site")
	}
	if err := build(root, root, metadata{}); err == nil {
		t.Fatal("repository root accepted as output")
	}
}
