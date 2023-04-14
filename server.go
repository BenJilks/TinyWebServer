package webserver

import (
    "fmt"
    log "github.com/sirupsen/logrus"
    "io"
    "mime"
    "net/http"
    "os"
    "path"
    "strings"
    "time"
)

const DefaultPort = 80
const DefaultTLSPort = 443

type PathType int

const (
    PathTypeNothing = PathType(iota)
    PathTypeFile
    PathTypeDirectory
)

type PathDescription struct {
    pathType PathType
    rawPath  string

    contentType  *string
    size         *int64
    lastModified *time.Time
}

type DoubleWriter struct {
    first  io.Writer
    second io.Writer
}

func (doubleWriter DoubleWriter) Write(data []byte) (int, error) {
    firstCount, err := doubleWriter.first.Write(data)
    if err != nil {
        return firstCount, err
    }

    secondCount, err := doubleWriter.second.Write(data[:firstCount])
    if err != nil {
        return secondCount, err
    }

    return firstCount, nil
}

func readPathDescription(filePath string) PathDescription {
    fileInfo, err := os.Stat(filePath)
    if err != nil {
        return PathDescription{
            pathType: PathTypeNothing,
            rawPath:  filePath,
        }
    }

    lastModified := fileInfo.ModTime()
    if fileInfo.IsDir() {
        return PathDescription{
            pathType:     PathTypeDirectory,
            rawPath:      filePath,
            lastModified: &lastModified,
        }
    }

    contentType := mime.TypeByExtension(path.Ext(filePath))
    size := fileInfo.Size()
    return PathDescription{
        pathType:     PathTypeFile,
        rawPath:      filePath,
        contentType:  &contentType,
        size:         &size,
        lastModified: &lastModified,
    }
}

func shouldGzipFile(description PathDescription, request *http.Request) bool {
    acceptEncoding := request.Header.Get("Accept-Encoding")
    if !strings.Contains(acceptEncoding, "gzip") {
        return false
    }

    // NOTE: Don't gzip video content.
    mimeType := mime.TypeByExtension(path.Ext(description.rawPath))
    if len(mimeType) >= 5 && mimeType[:5] == "video" {
        return false
    }

    return true
}

func serveFile(
    response http.ResponseWriter,
    request *http.Request,
    description PathDescription,
    gzipCache *GzipFileCache,
) {
    if !shouldGzipFile(description, request) {
        http.ServeFile(response, request, description.rawPath)
        return
    }

    if err := gzipCache.serveGzipFile(response, description); err != nil {
        log.WithError(err).
            WithField("file", description.rawPath).
            Error("Could not serve file gzipped")
        http.ServeFile(response, request, description.rawPath)
    }
}

func firstValidIndexPath(directoryPath string) PathDescription {
    validIndexFiles := []string{
        "index.html",
        "index.htm",
    }

    for _, indexFile := range validIndexFiles {
        indexPath := path.Join(directoryPath, indexFile)
        description := readPathDescription(indexPath)
        if description.pathType == PathTypeFile {
            return description
        }
    }

    return PathDescription{
        pathType: PathTypeNothing,
    }
}

func serveDirectory(
    response http.ResponseWriter,
    request *http.Request,
    directoryPath string,
    gzipCache *GzipFileCache,
) {
    if description := firstValidIndexPath(directoryPath); description.pathType != PathTypeNothing {
        serveFile(response, request, description, gzipCache)
        return
    }

    http.ServeFile(response, request, directoryPath)
}

func webHandler(config WebServerConfig) http.HandlerFunc {
    var gzipCache GzipFileCache
    if config.EnableGzip {
        gzipCache = createGzipFileCache(config.ServerName)
    }

    return func(response http.ResponseWriter, request *http.Request) {
        response.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
        response.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")

        url := request.URL.Path
        log.WithField("url", url).
            Trace("Got request")

        filePath := path.Join(config.StaticFilesPath, url)
        if !config.EnableGzip {
            http.ServeFile(response, request, filePath)
            return
        }

        description := readPathDescription(filePath)
        switch description.pathType {
        case PathTypeNothing:
            http.NotFound(response, request)
        case PathTypeFile:
            serveFile(response, request, description, &gzipCache)
        case PathTypeDirectory:
            serveDirectory(response, request, filePath, &gzipCache)
        }
    }
}

func displayWebAddress(address string, port uint, useTLS bool) string {
    protocol := "http"
    if useTLS {
        protocol = "https"
    }

    if (useTLS && port == DefaultTLSPort) || (!useTLS && port == DefaultPort) {
        return fmt.Sprintf("%s://%s", protocol, address)
    } else {
        return fmt.Sprintf("%s://%s:%d", protocol, address, port)
    }
}

func StartWebServer(config WebServerConfig) {
    useTLS := config.CertFilePath != "" && config.KeyFilePath != ""
    config.log(useTLS)

    bindAddress := fmt.Sprintf("%s:%d", config.Address, config.Port)
    handler := webHandler(config)
    log.WithFields(log.Fields{
        "address": displayWebAddress(config.Address, config.Port, useTLS),
        "TLS":     useTLS,
    }).Info("Started listening for connections")

    var err error
    if useTLS {
        err = http.ListenAndServeTLS(bindAddress,
            config.CertFilePath, config.KeyFilePath, handler)
    } else {
        err = http.ListenAndServe(bindAddress, handler)
    }

    if err != nil {
        log.Fatal(err)
    }
}
