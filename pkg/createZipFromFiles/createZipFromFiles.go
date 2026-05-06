package createzipfromfiles

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

func CreateZipFromFiles(zipFile *os.File, files []string) error {
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, filePath := range files {
		fileToZip, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer fileToZip.Close()

		info, err := fileToZip.Stat()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.Base(filePath)
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if _, err := io.Copy(writer, fileToZip); err != nil {
			return err
		}
	}

	return nil
}
