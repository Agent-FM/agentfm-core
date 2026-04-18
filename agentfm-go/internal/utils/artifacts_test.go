package utils

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestZipDirectory_HappyPath verifies that ZipDirectory walks a nested
// directory and writes every non-directory file into the archive with its
// relative path preserved. Multi-file + nested structure is the common case
// for AgentFM agent artifacts (e.g. PDFs alongside JSON logs).
func TestZipDirectory_HappyPath(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()

	files := map[string]string{
		"a.txt":          "hello",
		"b.txt":          "world",
		"sub/c.txt":      "nested",
		"sub/deep/d.txt": "deeper",
	}
	for name, content := range files {
		full := filepath.Join(srcDir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	destZip := filepath.Join(t.TempDir(), "out.zip")
	if err := ZipDirectory(srcDir, destZip); err != nil {
		t.Fatalf("ZipDirectory: %v", err)
	}

	rc, err := zip.OpenReader(destZip)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	t.Cleanup(func() { _ = rc.Close() })

	got := make(map[string]string)
	for _, f := range rc.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rdr, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rdr)
		_ = rdr.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		got[f.Name] = string(data)
	}

	for name, content := range files {
		if got[name] != content {
			t.Errorf("zip[%s] = %q, want %q", name, got[name], content)
		}
	}
}

// TestZipDirectory_MissingSource asserts we fail loudly when the source
// directory doesn't exist. Silent success would leave the Boss waiting
// forever for a zip that was never produced.
func TestZipDirectory_MissingSource(t *testing.T) {
	t.Parallel()
	destZip := filepath.Join(t.TempDir(), "out.zip")
	err := ZipDirectory("/this/really/does/not/exist", destZip)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "source directory") {
		t.Errorf("want wrapped source-directory error, got: %v", err)
	}
}

// TestZipDirectory_SourceIsFile covers the "user passed a file instead of a
// directory" footgun. Function checks info.IsDir() explicitly.
func TestZipDirectory_SourceIsFile(t *testing.T) {
	t.Parallel()
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	destZip := filepath.Join(t.TempDir(), "out.zip")
	err := ZipDirectory(f, destZip)
	if err == nil {
		t.Fatal("expected error for file-as-source, got nil")
	}
}

// TestZipDirectory_EmptyDir verifies zipping an empty directory produces a
// valid but empty zip. This matches the real-world case where the worker's
// sandbox dumped no files into /tmp/output.
func TestZipDirectory_EmptyDir(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	destZip := filepath.Join(t.TempDir(), "empty.zip")
	if err := ZipDirectory(srcDir, destZip); err != nil {
		t.Fatalf("ZipDirectory: %v", err)
	}
	rc, err := zip.OpenReader(destZip)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	t.Cleanup(func() { _ = rc.Close() })
	if len(rc.File) != 0 {
		names := make([]string, 0, len(rc.File))
		for _, f := range rc.File {
			names = append(names, f.Name)
		}
		sort.Strings(names)
		t.Errorf("expected 0 entries, got %d: %v", len(rc.File), names)
	}
}

// TestZipDirectory_UnwritableDest exercises the dest-creation error path.
// os.Create returns an error if the parent directory doesn't exist.
func TestZipDirectory_UnwritableDest(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	destZip := filepath.Join(t.TempDir(), "nonexistent-parent", "out.zip")
	err := ZipDirectory(srcDir, destZip)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create zip") {
		t.Errorf("want wrapped create-zip error, got: %v", err)
	}
}

// TestZipDirectory_PreservesDirectoryEntries asserts subdirectory entries
// are emitted with the trailing slash the zip spec requires. Some zip
// readers refuse to extract files whose parent directories are implicit.
func TestZipDirectory_PreservesDirectoryEntries(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "x.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	destZip := filepath.Join(t.TempDir(), "out.zip")
	if err := ZipDirectory(srcDir, destZip); err != nil {
		t.Fatalf("ZipDirectory: %v", err)
	}
	rc, err := zip.OpenReader(destZip)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	t.Cleanup(func() { _ = rc.Close() })

	var foundDirEntry bool
	for _, f := range rc.File {
		if f.Name == "sub/" {
			foundDirEntry = true
			break
		}
	}
	if !foundDirEntry {
		names := make([]string, 0, len(rc.File))
		for _, f := range rc.File {
			names = append(names, f.Name)
		}
		t.Errorf("expected 'sub/' entry in zip, got: %v", names)
	}
}
