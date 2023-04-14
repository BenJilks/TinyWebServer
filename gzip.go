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

type CachedGzipFile struct {
    Time         time.Time
    BeingWritten bool
    Size         int64
}

type GzipFileCache struct {
    tempDirectory string
    cache         map[string]CachedGzipFile
    mutex         sync.Mutex
}

func createGzipFileCache(name string) GzipFileCache {
    tempDirectory := path.Join(os.TempDir(), name)
    _ = os.MkdirAll(tempDirectory, os.ModeDir|os.ModePerm)

    log.WithField("cache-path", tempDirectory).
        Info("Using gzip cache")

    return GzipFileCache{
        tempDirectory: tempDirectory,
        cache:         map[string]CachedGzipFile{},
        mutex:         sync.Mutex{},
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

    writer := gzip.NewWriter(&DoubleWriter{
        first:  gzippedFile,
        second: response,
    })
    defer writer.Close()

    response.Header().Set("Content-Encoding", "gzip")
    return io.Copy(writer, originalFile)
}

func serveCachedGzippedFile(response http.ResponseWriter, filePath string, size int64) error {
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }

    response.Header().Set("Content-Length", fmt.Sprint(size))
    response.Header().Set("Content-Encoding", "gzip")
    _, err = io.Copy(response, file)
    return err
}

func (fileCache *GzipFileCache) getCachedGzippedFile(description PathDescription) (string, *CachedGzipFile) {
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

func (fileCache *GzipFileCache) cacheAndServeFile(
    response http.ResponseWriter,
    description PathDescription,
    gzippedFilePath string,
) error {
    log.WithField("file", description.rawPath).
        Info("Updating gzip cache")

    fileCache.mutex.Lock()
    fileCache.cache[description.rawPath] = CachedGzipFile{
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
    fileCache.cache[description.rawPath] = CachedGzipFile{
        Time:         *description.lastModified,
        BeingWritten: false,
        Size:         size,
    }
    fileCache.mutex.Unlock()
    return nil
}

func (fileCache *GzipFileCache) serveGzipFile(
    response http.ResponseWriter,
    description PathDescription,
) error {
    if description.pathType != PathTypeFile {
        return errors.New("file doesn't exist")
    }

    response.Header().Set("Content-Type", *description.contentType)
    gzippedFilePath, cachedFile := fileCache.getCachedGzippedFile(description)
    if cachedFile != nil {
        if cachedFile.BeingWritten {
            return errors.New("file currently being cached")
        }
        return serveCachedGzippedFile(response, gzippedFilePath, cachedFile.Size)
    }

    return fileCache.cacheAndServeFile(
        response, description, gzippedFilePath)
}
