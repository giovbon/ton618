package extract

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// ── EPUB ────────────────────────────────────────────────────────
//
// Um EPUB é um ZIP com:
//   - META-INF/container.xml → aponta para o OPF (package document);
//   - OPF → manifest (href de cada capítulo) + spine (ordem de leitura);
//   - um XHTML por capítulo.
//
// Se o container/OPF estiver corrompido, cai no fallback: todos os
// .xhtml/.html do ZIP em ordem alfabética (melhor que devolver nada).

type epubContainer struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type epubPackage struct {
	Metadata struct {
		Title string `xml:"title"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID        string `xml:"id,attr"`
			Href      string `xml:"href,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Refs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

// EPUB extrai o texto de um arquivo .epub.
func EPUB(filePath string) (*Result, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("abrir epub: %w", err)
	}
	defer zr.Close()

	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[normalizeZipName(f.Name)] = f
	}

	title, spine := epubSpine(files)
	if len(spine) == 0 {
		spine = epubFallbackSpine(zr.File)
	}

	res := &Result{Title: strings.TrimSpace(title)}
	remaining := MaxTextBytes
	for _, name := range spine {
		f := files[name]
		if f == nil {
			continue
		}
		chapterTitle, text, truncated := readXHTMLChapter(f, remaining)
		if truncated {
			res.Truncated = true
		}
		remaining -= len(text)
		text = normalizeText(text)
		if strings.TrimSpace(text) == "" {
			continue
		}
		res.Chapters = append(res.Chapters, Chapter{Title: chapterTitle, Text: text})
		if remaining <= 0 {
			res.Truncated = true
			break
		}
	}

	if err := emptyResult(res); err != nil {
		return nil, err
	}
	return res, nil
}

// epubSpine lê container.xml + OPF e devolve título e os caminhos dos
// capítulos na ordem do spine.
func epubSpine(files map[string]*zip.File) (title string, spine []string) {
	containerFile := files["META-INF/container.xml"]
	if containerFile == nil {
		return "", nil
	}
	var container epubContainer
	if err := unmarshalZipXML(containerFile, &container); err != nil {
		return "", nil
	}
	if len(container.Rootfiles) == 0 {
		return "", nil
	}

	opfPath := normalizeZipName(container.Rootfiles[0].FullPath)
	opfFile := files[opfPath]
	if opfFile == nil {
		return "", nil
	}
	var pkg epubPackage
	if err := unmarshalZipXML(opfFile, &pkg); err != nil {
		return "", nil
	}

	opfDir := path.Dir(opfPath)
	if opfDir == "." {
		opfDir = ""
	}
	title = strings.TrimSpace(pkg.Metadata.Title)

	byID := make(map[string]string, len(pkg.Manifest.Items))
	for _, item := range pkg.Manifest.Items {
		if !isXHTMLHref(item.Href) {
			continue
		}
		byID[item.ID] = resolveZipHref(opfDir, item.Href)
	}

	for _, ref := range pkg.Spine.Refs {
		if name := byID[ref.IDRef]; name != "" {
			spine = append(spine, name)
		}
	}
	return title, spine
}

// epubFallbackSpine lista os XHTML do ZIP em ordem alfabética (usado quando o
// container/OPF não podem ser lidos).
func epubFallbackSpine(entries []*zip.File) []string {
	var names []string
	for _, f := range entries {
		n := normalizeZipName(f.Name)
		if !isXHTMLHref(n) {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// readXHTMLChapter extrai título e texto de um capítulo XHTML, respeitando o
// teto global de bytes (remaining). O título não é duplicado no texto.
func readXHTMLChapter(f *zip.File, remaining int) (title, text string, truncated bool) {
	rc, err := f.Open()
	if err != nil {
		return "", "", false
	}
	defer rc.Close()

	doc, err := html.Parse(rc)
	if err != nil {
		return "", "", false
	}

	var b strings.Builder
	used := 0
	write := func(s string) {
		if s == "" {
			return
		}
		if used+len(s) > remaining {
			if remaining > used {
				b.WriteString(truncateUTF8(s, remaining-used))
			}
			truncated = true
			return
		}
		b.WriteString(s)
		used += len(s)
	}

	var headTitle string
	var walk func(n *html.Node, pre bool)
	walk = func(n *html.Node, pre bool) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			switch tag {
			case "script", "style", "svg", "noscript", "nav":
				return
			case "head":
				// <title> fica no head; aproveita como título de reserva.
				if headTitle == "" {
					if t := findFirstText(n, "title"); t != "" {
						headTitle = t
					}
				}
				return
			case "title":
				if headTitle == "" {
					headTitle = textContent(n)
				}
				return
			case "h1", "h2", "h3":
				// Primeiro heading do corpo vira o título do capítulo e sai do
				// texto (senão apareceria duplicado logo abaixo do heading da
				// nota).
				if title == "" {
					if t := textContent(n); t != "" {
						title = t
						return
					}
				}
				write("\n\n")
			case "br":
				write("\n")
				return
			case "li":
				write("\n- ")
			case "td", "th":
				write(" | ")
			}
			pre = pre || tag == "pre"
		}

		if n.Type == html.TextNode {
			s := n.Data
			if !pre {
				s = strings.Join(strings.Fields(strings.ReplaceAll(s, "\u00a0", " ")), " ")
				if s == "" {
					return
				}
				if b.Len() > 0 {
					s = " " + s
				}
			}
			write(s)
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, pre)
		}
		if n.Type == html.ElementNode && isBlockTag(strings.ToLower(n.Data)) {
			write("\n\n")
		}
	}

	walk(doc, false)
	text = b.String()
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSpace(headTitle)
	}
	return title, text, truncated
}

func findFirstText(n *html.Node, tag string) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.ToLower(c.Data) == tag {
			return textContent(c)
		}
		if t := findFirstText(c, tag); t != "" {
			return t
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func isBlockTag(tag string) bool {
	switch tag {
	case "p", "div", "section", "article", "header", "footer", "aside",
		"blockquote", "pre", "table", "tr", "ul", "ol", "li", "figure",
		"figcaption", "h1", "h2", "h3", "h4", "h5", "h6", "hr":
		return true
	}
	return false
}

func isXHTMLHref(href string) bool {
	h := strings.ToLower(href)
	if i := strings.IndexByte(h, '#'); i >= 0 {
		h = h[:i]
	}
	return strings.HasSuffix(h, ".xhtml") || strings.HasSuffix(h, ".html") || strings.HasSuffix(h, ".htm")
}

func normalizeZipName(name string) string {
	return strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(name, "\\", "/")), "/")
}

// resolveZipHref resolve o href do manifest relativo ao diretório do OPF e
// normaliza para o mesmo formato das entradas do ZIP.
func resolveZipHref(opfDir, href string) string {
	href = strings.TrimSpace(href)
	if i := strings.IndexByte(href, '#'); i >= 0 {
		href = href[:i]
	}
	if unescaped, err := url.PathUnescape(href); err == nil {
		href = unescaped
	}
	return normalizeZipName(path.Join(opfDir, href))
}

func unmarshalZipXML(f *zip.File, v any) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	// 4 MiB é folgado para container.xml/OPF.
	data, err := io.ReadAll(io.LimitReader(rc, 4<<20))
	if err != nil {
		return err
	}
	return xml.Unmarshal(data, v)
}
