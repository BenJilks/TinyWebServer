package webserver

import (
	"compress/gzip"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type cachedGzipFile struct {
	Time         time.Time
	BeingWritten bool
	Size         int64
}

type gzipFileCache struct {
	tempDirectory string
	cache         map[string]cachedGzipFile
	mutex         sync.Mutex
	gmt           *time.Location
}

type doubleWriter struct {
	first  io.Writer
	second io.Writer
}

func (writer doubleWriter) Write(data []byte) (int, error) {
	firstCount, err := writer.first.Write(data)
	if err != nil {
		return firstCount, err
	}

	secondCount, err := writer.second.Write(data[:firstCount])
	if err != nil {
		return secondCount, err
	}

	return firstCount, nil
}

func createGzipFileCache(name string) gzipFileCache {
	tempDirectory := path.Join(os.TempDir(), name)
	_ = os.MkdirAll(tempDirectory, os.ModeDir|os.ModePerm)

	log.WithField("cache-path", tempDirectory).
		Info("Using gzip cache")

	location, err := time.LoadLocation("GMT")
	if err != nil {
		panic(err)
	}

	return gzipFileCache{
		tempDirectory: tempDirectory,
		cache:         map[string]cachedGzipFile{},
		mutex:         sync.Mutex{},
		gmt:           location,
	}
}

func gzipAndServeFile(filePath string, gzippedFilePath string, response http.ResponseWriter) (int64, error) {
	originalFile, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer originalFile.Close()

	gzippedFile, err := os.Create(gzippedFilePath)
	if err != nil {
		return 0, err
	}
	defer gzippedFile.Close()

	writer := gzip.NewWriter(&doubleWriter{
		first:  gzippedFile,
		second: response,
	})
	defer writer.Close()

	response.Header().Set("Content-Encoding", "gzip")
	return io.Copy(writer, originalFile)
}

func serveCachedGzippedFile(response http.ResponseWriter, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	response.Header().Set("Content-Encoding", "gzip")
	_, err = io.Copy(response, file)
	return err
}

func (fileCache *gzipFileCache) getCachedGzippedFile(description pathDescription) (string, *cachedGzipFile) {
	cacheName := strings.ReplaceAll(description.rawPath, "/", "_")
	cacheName = strings.ReplaceAll(cacheName, ".", "_")
	gzippedFilePath := path.Join(fileCache.tempDirectory, cacheName+".gz")

	fileCache.mutex.Lock()
	cachedFile, inCache := fileCache.cache[description.rawPath]
	fileCache.mutex.Unlock()

	if inCache && !description.lastModified.After(cachedFile.Time) {
		return gzippedFilePath, &cachedFile
	}

	return gzippedFilePath, nil
}

func (fileCache *gzipFileCache) cacheAndServeFile(
	response http.ResponseWriter,
	description pathDescription,
	gzippedFilePath string,
) error {
	log.WithField("file", description.rawPath).
		Info("Updating gzip cache")

	fileCache.mutex.Lock()
	fileCache.cache[description.rawPath] = cachedGzipFile{
		Time:         *description.lastModified,
		BeingWritten: true,
	}
	fileCache.mutex.Unlock()

	size, err := gzipAndServeFile(description.rawPath, gzippedFilePath, response)
	if err != nil {
		delete(fileCache.cache, description.rawPath)
		return err
	}

	fileCache.mutex.Lock()
	fileCache.cache[description.rawPath] = cachedGzipFile{
		Time:         *description.lastModified,
		BeingWritten: false,
		Size:         size,
	}
	fileCache.mutex.Unlock()
	return nil
}

func (fileCache *gzipFileCache) serveGzipFile(
	response http.ResponseWriter,
	description pathDescription,
) error {
	if description.pathType != pathTypeFile {
		return errors.New("file doesn't exist")
	}

	response.Header().Set("Content-Type", *description.contentType)
	gzippedFilePath, cachedFile := fileCache.getCachedGzippedFile(description)
	if cachedFile == nil {
		return fileCache.cacheAndServeFile(
			response, description, gzippedFilePath)
	}

	if cachedFile.BeingWritten {
		return errors.New("file currently being cached")
	}

	response.Header().Set("Content-Length", fmt.Sprint(cachedFile.Size))
	response.Header().Set("Last-Modified", cachedFile.Time.In(fileCache.gmt).
		Format("Mon, 2 Jan 2006 15:04:05 MST"))
	return serveCachedGzippedFile(response, gzippedFilePath)
}
