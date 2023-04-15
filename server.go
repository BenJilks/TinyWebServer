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

type pathType int

const (
	pathTypeNothing = pathType(iota)
	pathTypeFile
	pathTypeDirectory
)

type pathDescription struct {
	pathType pathType
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

func readPathDescription(filePath string) pathDescription {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return pathDescription{
			pathType: pathTypeNothing,
			rawPath:  filePath,
		}
	}

	lastModified := fileInfo.ModTime()
	if fileInfo.IsDir() {
		return pathDescription{
			pathType:     pathTypeDirectory,
			rawPath:      filePath,
			lastModified: &lastModified,
		}
	}

	contentType := mime.TypeByExtension(path.Ext(filePath))
	size := fileInfo.Size()
	return pathDescription{
		pathType:     pathTypeFile,
		rawPath:      filePath,
		contentType:  &contentType,
		size:         &size,
		lastModified: &lastModified,
	}
}

func shouldGzipFile(description pathDescription, request *http.Request) bool {
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
	description pathDescription,
	gzipCache *gzipFileCache,
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

func firstValidIndexPath(directoryPath string) pathDescription {
	validIndexFiles := []string{
		"index.html",
		"index.htm",
	}

	for _, indexFile := range validIndexFiles {
		indexPath := path.Join(directoryPath, indexFile)
		description := readPathDescription(indexPath)
		if description.pathType == pathTypeFile {
			return description
		}
	}

	return pathDescription{
		pathType: pathTypeNothing,
	}
}

func serveDirectory(
	response http.ResponseWriter,
	request *http.Request,
	directoryPath string,
	gzipCache *gzipFileCache,
) {
	if description := firstValidIndexPath(directoryPath); description.pathType != pathTypeNothing {
		serveFile(response, request, description, gzipCache)
		return
	}

	http.ServeFile(response, request, directoryPath)
}

func Handler(config Config) http.HandlerFunc {
	var gzipCache gzipFileCache
	if config.EnableGzip {
		gzipCache = createGzipFileCache(config.ServerName)
	}

	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		response.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")

		url := request.URL.Path
		filePath := path.Clean(path.Join(config.StaticFilesPath, url))
		log.WithFields(log.Fields{"url": url, "path": filePath}).
			Trace("Got request")

		if !config.EnableGzip {
			http.ServeFile(response, request, filePath)
			return
		}

		description := readPathDescription(filePath)
		switch description.pathType {
		case pathTypeNothing:
			http.NotFound(response, request)
		case pathTypeFile:
			serveFile(response, request, description, &gzipCache)
		case pathTypeDirectory:
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

func httpToHttpsRedirectService(config Config) {
	log.Info("Starting http to https redirect service")

	bindAddress := fmt.Sprintf("%s:80", config.Address)
	err := http.ListenAndServe(bindAddress, http.HandlerFunc(
		func(response http.ResponseWriter, request *http.Request) {
			redirectUrl := fmt.Sprintf("https://%s:%d%s", request.Host, config.Port, request.URL.Path)
			if len(request.URL.RawQuery) > 0 {
				redirectUrl = fmt.Sprintf("%s?%s", redirectUrl, request.URL.RawQuery)
			}

			log.WithField("url", redirectUrl).
				Trace("Got http request, redirecting to https")
			http.Redirect(response, request, redirectUrl, http.StatusPermanentRedirect)
		}))

	if err != nil {
		log.Fatal(err)
	}
}

func Listen(config Config, handler http.HandlerFunc) error {
	useTLS := config.CertFilePath != "" && config.KeyFilePath != ""
	config.log(useTLS)

	bindAddress := fmt.Sprintf("%s:%d", config.Address, config.Port)
	log.WithFields(log.Fields{
		"address": displayWebAddress(config.Address, config.Port, useTLS),
		"TLS":     useTLS,
	}).Info("Started listening for connections")

	if useTLS {
		if config.EnableHttpToHttps {
			go httpToHttpsRedirectService(config)
		}

		return http.ListenAndServeTLS(bindAddress,
			config.CertFilePath, config.KeyFilePath, handler)
	} else {
		if config.EnableHttpToHttps {
			log.Warning("Can't enable http to https redirect, as TLS is not enabled")
		}

		return http.ListenAndServe(bindAddress, handler)
	}
}
