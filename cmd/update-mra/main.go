// update-mra downloads the latest MRA zip from the given URL, extracts the
// target device class JSONs, and copies them into echonet/spec/mra/.
//
// Run from the repository root after finding the latest zip URL via
// https://echonet.jp/spec_g/ → Appendix MRA page:
//
//	go run ./cmd/update-mra <MRA_zip_URL> <version_string>
//
// Example:
//
//	go run ./cmd/update-mra \
//	  https://echonet.jp/wp/wp-content/uploads/pdf/General/Standard/MRA/MRA_v1.4.0.zip \
//	  MRA_v1.4.0
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// device class EOJ codes to include (current 14 classes)
var targetDevices = []string{
	"0x0011", "0x0012", "0x001B", "0x0130",
	"0x026B", "0x0279", "0x027C", "0x027D",
	"0x027E", "0x0287", "0x0288", "0x0290", "0x02A1",
}

const destDir = "echonet/spec/mra"

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: update-mra <zip_url> <version>")
		fmt.Fprintln(os.Stderr, "example: update-mra https://echonet.jp/.../MRA_v1.4.0.zip MRA_v1.4.0")
		os.Exit(1)
	}
	zipURL := os.Args[1]
	version := os.Args[2]

	versionFile := filepath.Join(destDir, "VERSION")
	if cur, err := os.ReadFile(versionFile); err == nil {
		lines := strings.SplitN(strings.TrimSpace(string(cur)), "\n", 2)
		if lines[0] == version {
			fmt.Printf("already up to date: %s\n", version)
			return
		}
		fmt.Printf("current: %s → updating to: %s\n", lines[0], version)
	}

	fmt.Printf("downloading %s ...\n", zipURL)
	data, err := download(zipURL)
	if err != nil {
		fatal("download failed: %v", err)
	}
	fmt.Printf("downloaded %d bytes\n", len(data))

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		fatal("open zip: %v", err)
	}

	// find the top-level MRA directory (MRA_vX.Y.Z/)
	var mraRoot string
	for _, f := range zr.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) >= 1 && strings.HasPrefix(parts[0], "MRA_") {
			mraRoot = parts[0] + "/"
			break
		}
	}
	if mraRoot == "" {
		fatal("could not find MRA_*/ directory inside zip")
	}
	fmt.Printf("MRA root in zip: %s\n", mraRoot)

	copyCount := 0

	// device classes
	for _, eoj := range targetDevices {
		src := mraRoot + "devices/" + eoj + ".json"
		dst := filepath.Join(destDir, eoj+".json")
		if err := extractFile(zr, src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}
		copyCount++
	}

	// node profile, super class, definitions, metadata
	extras := map[string]string{
		mraRoot + "nodeProfile/0x0EF0.json": filepath.Join(destDir, "0x0EF0.json"),
		mraRoot + "superClass/0x0000.json":  filepath.Join(destDir, "0x0000.json"),
		mraRoot + "definitions/definitions.json": filepath.Join(destDir, "definitions.json"),
		mraRoot + "metaData.json":           filepath.Join(destDir, "metaData.json"),
	}
	for src, dst := range extras {
		if err := extractFile(zr, src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}
		copyCount++
	}

	// update VERSION
	versionContent := fmt.Sprintf("%s\n%s\n", version, zipURL)
	if err := os.WriteFile(versionFile, []byte(versionContent), 0644); err != nil {
		fatal("write VERSION: %v", err)
	}

	fmt.Printf("copied %d files → %s\n", copyCount, destDir)
	fmt.Println("next: go build -o el-mcp-server . && verify with search_device_class / list_epc / get_epc_detail")
}

func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
}

func extractFile(zr *zip.Reader, name, dest string) error {
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %s: %w", name, err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		return nil
	}
	return fmt.Errorf("%s not found in zip", name)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
