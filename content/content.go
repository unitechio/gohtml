package content

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Content is an interface used for putting the content into Client Query.
type Content interface {
	ContentType() string
	Method() string
	Data() []byte
}

// -------------------- ZIP DIRECTORY --------------------

type zipDirectory struct {
	buffer bytes.Buffer
	writer *zip.Writer
}

func (z *zipDirectory) zipPath(dirPath, basePath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("reading directory '%s' failed: %w", dirPath, err)
	}

	builder := &strings.Builder{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return err
		}

		filePath, zipPath := z.zipBasePaths(info, dirPath, basePath, entry.IsDir(), builder)

		if entry.IsDir() {
			if err := z.zipPath(filePath, zipPath); err != nil {
				return err
			}
			continue
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		writer, err := z.writer.Create(zipPath)
		if err != nil {
			return err
		}

		if _, err = writer.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func (z *zipDirectory) zipBasePaths(info os.FileInfo, dirPath, basePath string, isDir bool, builder *strings.Builder) (string, string) {
	// local path
	builder.WriteString(dirPath)
	if !strings.HasSuffix(dirPath, "/") {
		builder.WriteRune('/')
	}
	builder.WriteString(info.Name())
	if isDir {
		builder.WriteRune('/')
	}
	localPath := builder.String()

	// zip entry path
	builder.Reset()
	builder.WriteString(basePath)
	builder.WriteString(info.Name())
	if isDir {
		builder.WriteRune('/')
	}
	zipPath := builder.String()

	builder.Reset()
	return localPath, zipPath
}

// Method implements Content interface.
func (z *zipDirectory) Method() string { return "dir" }

// ContentType implements Content interface.
func (z *zipDirectory) ContentType() string { return "application/zip" }

// Data implements Content interface.
func (z *zipDirectory) Data() []byte { return z.buffer.Bytes() }

// NewZipDirectory creates new zip compressed file that recursively reads the directory at the 'dirPath'.
func NewZipDirectory(dirPath string) (Content, error) {
	zd := &zipDirectory{buffer: bytes.Buffer{}}
	zd.writer = zip.NewWriter(&zd.buffer)

	if err := zd.zipPath(dirPath, ""); err != nil {
		return nil, err
	}
	if err := zd.writer.Close(); err != nil {
		return nil, err
	}
	return zd, nil
}

// -------------------- HTML FILE --------------------

type htmlFile struct {
	buffer bytes.Buffer
}

// NewHTMLFile creates new Content htmlFile for provided input path.
func NewHTMLFile(path string) (Content, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hf := &htmlFile{buffer: bytes.Buffer{}}
	if _, err := hf.buffer.ReadFrom(f); err != nil {
		return nil, err
	}
	return hf, nil
}

// Method implements Content interface.
func (h *htmlFile) Method() string { return "html" }

// ContentType implements Content interface.
func (h *htmlFile) ContentType() string { return "text/html" }

// Data implements Content interface.
func (h *htmlFile) Data() []byte { return h.buffer.Bytes() }

// -------------------- STRING CONTENT --------------------

type StringContent struct {
	html string
}

// NewStringContent creates a new StringContent.
func NewStringContent(html string) (*StringContent, error) {
	return &StringContent{html: html}, nil
}

// Method implements Content interface.
func (s *StringContent) Method() string { return "html" }

// ContentType implements Content interface.
func (s *StringContent) ContentType() string { return "text/html" }

// Data implements Content interface.
func (s *StringContent) Data() []byte { return []byte(s.html) }

// -------------------- WEB URL --------------------

type webURL struct {
	path string
}

// NewWebURL creates new Content webURL for provided input URL path.
func NewWebURL(path string) (Content, error) {
	if _, err := url.Parse(path); err != nil {
		return nil, err
	}
	return &webURL{path: path}, nil
}

// Method implements Content interface.
func (w *webURL) Method() string { return "web" }

// ContentType implements Content interface.
func (w *webURL) ContentType() string { return "text/plain" }

// Data implements Content interface.
func (w *webURL) Data() []byte { return []byte(w.path) }
