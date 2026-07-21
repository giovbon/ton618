// Package staticver fornece cache automático de versão para arquivos estáticos
// usando hashes SHA256 do conteúdo. Elimina a necessidade de cache-busters manuais (?v=).
package staticver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Cache armazena hashes SHA256 dos arquivos estáticos para uso como ETags.
type Cache struct {
	mu     sync.RWMutex
	hashes map[string]string // caminho relativo -> hash hex(primeiros 12 chars)
	dir    string
}

// NewCache escaneia o diretório estático e pré-computa hashes de todos os arquivos.
func NewCache(staticDir string) (*Cache, error) {
	c := &Cache{
		hashes: make(map[string]string),
		dir:    staticDir,
	}
	if err := c.scan(); err != nil {
		return nil, fmt.Errorf("staticver: erro ao escanear %s: %w", staticDir, err)
	}
	return c, nil
}

func (c *Cache) scan() error {
	return filepath.Walk(c.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Pula arquivos pré-comprimidos
		if strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".br") {
			return nil
		}

		// Só hasheia arquivos que podem ser cacheados pelo browser
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".js" && ext != ".css" && ext != ".html" && ext != ".svg" && ext != ".woff2" && ext != ".woff" && ext != ".ttf" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		hash := sha256.Sum256(data)
		hexHash := hex.EncodeToString(hash[:])

		relPath, err := filepath.Rel(c.dir, path)
		if err != nil {
			relPath = path
		}

		c.mu.Lock()
		c.hashes[relPath] = hexHash[:12] // 12 chars é suficiente para evitar colisão
		c.mu.Unlock()

		return nil
	})
}

// ETag retorna o valor ETag para o caminho relativo informado.
// Ex: ETag("js/app.js") → `"a1b2c3d4e5f6"`
func (c *Cache) ETag(relPath string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if h, ok := c.hashes[relPath]; ok {
		return `"` + h + `"`
	}
	return ""
}

// VersionedPath retorna o caminho com o hash como query string,
// ex: "/static/js/app.js?v=a1b2c3d4e5f6"
func (c *Cache) VersionedPath(relPath string) string {
	c.mu.RLock()
	hash, ok := c.hashes[relPath]
	c.mu.RUnlock()
	if !ok {
		return "/static/" + relPath
	}
	return fmt.Sprintf("/static/%s?v=%s", relPath, hash)
}

// Handler é um http.Handler que serve arquivos estáticos com ETags e cache automático.
// Usa o cache de hashes para gerar ETags e responde 304 Not Modified quando o
// conteúdo não mudou, sem precisar de ?v= manual.
func (c *Cache) Handler(staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Limpa o path
		filePath := filepath.Clean(r.URL.Path)
		if strings.HasPrefix(filePath, "/") {
			filePath = filePath[1:]
		}
		fullPath := filepath.Join(staticDir, filePath)

		// Info do arquivo
		info, err := os.Stat(fullPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if info.IsDir() {
			http.NotFound(w, r)
			return
		}

		// Obtém ETag do cache
		etag := c.ETag(filePath)

		// Se tem ETag, configura headers de cache
		if etag != "" {
			w.Header().Set("ETag", etag)
			// Cache no browser por 1 ano, mas revalida com ETag se necessário
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

			// Verifica If-None-Match (browser perguntando se mudou)
			if match := r.Header.Get("If-None-Match"); match == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		} else {
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		}

		// Determina Content-Type
		ext := filepath.Ext(filePath)
		contentType := mimeTypeByExt(ext)
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}

		// Tenta servir pré-comprimido
		acceptEncoding := r.Header.Get("Accept-Encoding")

		// 1. Brotli
		if strings.Contains(acceptEncoding, "br") {
			brPath := fullPath + ".br"
			if _, err := os.Stat(brPath); err == nil {
				w.Header().Set("Content-Encoding", "br")
				http.ServeFile(w, r, brPath)
				return
			}
		}

		// 2. Gzip
		if strings.Contains(acceptEncoding, "gzip") {
			gzPath := fullPath + ".gz"
			if _, err := os.Stat(gzPath); err == nil {
				w.Header().Set("Content-Encoding", "gzip")
				http.ServeFile(w, r, gzPath)
				return
			}
		}

		// 3. Fallback: arquivo original
		http.ServeFile(w, r, fullPath)
	})
}

func mimeTypeByExt(ext string) string {
	switch ext {
	case ".js":
		return "application/javascript"
	case ".css":
		return "text/css"
	case ".html":
		return "text/html"
	case ".svg":
		return "image/svg+xml"
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".ttf":
		return "font/ttf"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}
