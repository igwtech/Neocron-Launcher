package addon

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// stageFetched downloads one FetchEntry, extracts it according to entry.Extract,
// and copies the declared files into cacheDir. Used at install time for addons
// that reference upstream binaries (e.g. dgVoodoo2 from GitHub releases).
//
// The temp dir is removed before return regardless of success.
func (m *Manager) stageFetched(entry FetchEntry, cacheDir string, report func(msg string)) error {
	if entry.From == "" {
		return fmt.Errorf("fetch: 'from' is required")
	}
	if !strings.HasPrefix(entry.From, "http://") && !strings.HasPrefix(entry.From, "https://") {
		return fmt.Errorf("fetch: 'from' must be http or https URL, got %q", entry.From)
	}

	tmp, err := os.MkdirTemp(m.DataDir, "addon-fetch-*")
	if err != nil {
		return fmt.Errorf("create fetch tmp: %w", err)
	}
	defer os.RemoveAll(tmp)

	if report != nil {
		report(fmt.Sprintf("Fetching %s", entry.From))
	}

	resp, err := http.Get(entry.From)
	if err != nil {
		return fmt.Errorf("download %s: %w", entry.From, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, entry.From)
	}

	extractedRoot := filepath.Join(tmp, "extracted")
	if err := os.MkdirAll(extractedRoot, 0755); err != nil {
		return err
	}

	switch entry.Extract {
	case "zip":
		archivePath := filepath.Join(tmp, "archive.zip")
		if err := writeBody(resp.Body, archivePath); err != nil {
			return err
		}
		if err := unzipTo(archivePath, extractedRoot); err != nil {
			return fmt.Errorf("unzip: %w", err)
		}
	case "tar.gz", "tgz":
		gr, gerr := gzip.NewReader(resp.Body)
		if gerr != nil {
			return fmt.Errorf("gzip: %w", gerr)
		}
		defer gr.Close()
		if err := untarTo(gr, extractedRoot); err != nil {
			return fmt.Errorf("untar: %w", err)
		}
	case "":
		// Raw single-file download — write under extracted/<basename>.
		base := path.Base(entry.From)
		if base == "" || base == "/" || base == "." {
			base = "download.bin"
		}
		dst := filepath.Join(extractedRoot, base)
		if err := writeBody(resp.Body, dst); err != nil {
			return err
		}
	default:
		return fmt.Errorf("fetch: unsupported extract format %q (want 'zip', 'tar.gz', or '')", entry.Extract)
	}

	for _, fe := range entry.Files {
		src := filepath.Join(extractedRoot, filepath.FromSlash(fe.Src))
		// Path-traversal guard: src must remain inside extractedRoot.
		if !strings.HasPrefix(filepath.Clean(src), filepath.Clean(extractedRoot)) {
			m.log("stageFetched: skipping out-of-archive src %q", fe.Src)
			continue
		}
		if _, err := os.Stat(src); err != nil {
			m.log("stageFetched: source %q not found in archive (%v)", fe.Src, err)
			if report != nil {
				report(fmt.Sprintf("Skipping %s (not in archive)", fe.Src))
			}
			continue
		}
		dst := filepath.Join(cacheDir, filepath.FromSlash(fe.Dst))
		if err := copyTree(src, dst); err != nil {
			return fmt.Errorf("stage %s -> %s: %w", fe.Src, fe.Dst, err)
		}
	}
	return nil
}

func writeBody(r io.Reader, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// unzipTo extracts a .zip archive to destDir. Path-traversal entries are
// silently skipped.
func unzipTo(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	for _, f := range r.File {
		target := filepath.Join(destDir, filepath.FromSlash(f.Name))
		// Path traversal guard.
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) &&
			filepath.Clean(target) != filepath.Clean(destDir) {
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, oerr := f.Open()
		if oerr != nil {
			return oerr
		}
		out, cerr := os.Create(target)
		if cerr != nil {
			rc.Close()
			return cerr
		}
		if _, ierr := io.Copy(out, rc); ierr != nil {
			rc.Close()
			out.Close()
			return ierr
		}
		rc.Close()
		out.Close()
	}
	return nil
}

// untarTo extracts a tar stream (already gzip-decoded) to destDir.
// Path-traversal entries are silently skipped.
func untarTo(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, filepath.FromSlash(header.Name))
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) &&
			filepath.Clean(target) != filepath.Clean(destDir) {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
}
