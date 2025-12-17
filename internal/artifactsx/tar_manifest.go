package artifactsx

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
)

const maxManifestBytes = 1 << 20

func ReadManifestFromTarGz(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть архив: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("не удалось прочитать tar: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		name := strings.TrimPrefix(hdr.Name, "./")
		if name != "manifest.json" {
			continue
		}

		if hdr.Size <= 0 {
			return nil, fmt.Errorf("manifest.json пустой")
		}
		if hdr.Size > maxManifestBytes {
			return nil, fmt.Errorf("manifest.json слишком большой")
		}

		lr := &io.LimitedReader{R: tr, N: maxManifestBytes + 1}
		b, err := io.ReadAll(lr)
		if err != nil {
			return nil, fmt.Errorf("не удалось прочитать manifest.json: %w", err)
		}
		if int64(len(b)) > maxManifestBytes {
			return nil, fmt.Errorf("manifest.json слишком большой")
		}
		return b, nil
	}

	return nil, fmt.Errorf("manifest.json не найден")
}
