package webserver

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

type Config struct {
	Address         string
	Port            uint
	StaticFilesPath string
	ServerName      string

	CertFilePath string
	KeyFilePath  string

	EnableHttpToHttps bool
	EnableGzip        bool
}

func (config *Config) log(useTLS bool) {
	fields := log.Fields{
		"address":      config.Address,
		"port":         config.Port,
		"static-files": config.StaticFilesPath,
		"name":         config.ServerName,
		"tls":          useTLS,

		"enable-http-to-https": config.EnableHttpToHttps,
		"enable-gzip":          config.EnableGzip,
	}

	if useTLS {
		fields["cert"] = config.CertFilePath
		fields["key"] = config.KeyFilePath
	}

	log.WithFields(fields).
		Trace("Using web server configuration")
}

func DefaultConfig() Config {
	return Config{
		Address:         "0.0.0.0",
		Port:            8080,
		StaticFilesPath: "/srv/www",
		ServerName:      "tiny-web-server",

		KeyFilePath:  "",
		CertFilePath: "",

		EnableHttpToHttps: false,
		EnableGzip:        true,
	}
}

func FileConfig(configFile *ini.File, config Config) Config {
	serverSection := configFile.Section("server")

	return Config{
		Address:         serverSection.Key("address").MustString(config.Address),
		Port:            serverSection.Key("port").MustUint(config.Port),
		StaticFilesPath: serverSection.Key("static").MustString(config.StaticFilesPath),
		ServerName:      serverSection.Key("name").MustString(config.ServerName),

		CertFilePath: serverSection.Key("cert").MustString(config.CertFilePath),
		KeyFilePath:  serverSection.Key("key").MustString(config.KeyFilePath),

		EnableHttpToHttps: serverSection.Key("http-to-https").MustBool(config.EnableHttpToHttps),
		EnableGzip:        serverSection.Key("gzip").MustBool(config.EnableGzip),
	}
}

func CommandLineConfig(config Config) Config {
	port := flag.Uint("port", config.Port, "Port")
	staticFilesPath := flag.String("static", config.StaticFilesPath, "Root path of static files")
	serverName := flag.String("name", config.ServerName, "Name of web server")

	certFile := flag.String("cert", config.CertFilePath, "TLS cert file")
	keyFile := flag.String("key", config.KeyFilePath, "TLS key file")

	enableHttpToHttps := flag.Bool("enable-http-to-https", config.EnableHttpToHttps, "Enable http to https redirect")
	disableGzip := flag.Bool("disable-gzip", !config.EnableGzip, "Disable Gzip compression")

	flag.Parse()
	return Config{
		Address:         config.Address,
		Port:            *port,
		StaticFilesPath: *staticFilesPath,
		ServerName:      *serverName,

		CertFilePath: *certFile,
		KeyFilePath:  *keyFile,

		EnableHttpToHttps: *enableHttpToHttps,
		EnableGzip:        !*disableGzip,
	}
}
