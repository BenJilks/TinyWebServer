package main

import (
    "flag"
    log "github.com/sirupsen/logrus"
    "gopkg.in/ini.v1"
)

type WebServerConfig struct {
    Address         string
    Port            uint
    StaticFilesPath string
    ServerName      string

    CertFilePath string
    KeyFilePath  string

    EnableGzip bool
}

func (config *WebServerConfig) log(useTLS bool) {
    fields := log.Fields{
        "address":      config.Address,
        "port":         config.Port,
        "static-files": config.StaticFilesPath,
        "name":         config.ServerName,
        "tls":          useTLS,
        "enable-gzip":  config.EnableGzip,
    }

    if useTLS {
        fields["cert"] = config.CertFilePath
        fields["key"] = config.KeyFilePath
    }

    log.WithFields(fields).
        Trace("Using web server configuration")
}

func DefaultWebServerConfig() WebServerConfig {
    return WebServerConfig{
        Address:         "0.0.0.0",
        Port:            8080,
        StaticFilesPath: "/srv/www",
        ServerName:      "tiny-web-server",

        KeyFilePath:  "",
        CertFilePath: "",

        EnableGzip: true,
    }
}

func FileWebServerConfig(configFile *ini.File, config WebServerConfig) WebServerConfig {
    serverSection := configFile.Section("server")

    return WebServerConfig{
        Address:         config.Address,
        Port:            serverSection.Key("port").MustUint(config.Port),
        StaticFilesPath: serverSection.Key("static").MustString(config.StaticFilesPath),
        ServerName:      serverSection.Key("name").MustString(config.ServerName),

        CertFilePath: serverSection.Key("cert").MustString(config.CertFilePath),
        KeyFilePath:  serverSection.Key("key").MustString(config.KeyFilePath),

        EnableGzip: serverSection.Key("gzip").MustBool(config.EnableGzip),
    }
}

func CommandLineWebServerConfig(config WebServerConfig) WebServerConfig {
    port := flag.Uint("port", config.Port, "Port")
    staticFilesPath := flag.String("static", config.StaticFilesPath, "Root path of static files")
    serverName := flag.String("name", config.ServerName, "Name of web server")

    certFile := flag.String("cert", config.CertFilePath, "TLS cert file")
    keyFile := flag.String("key", config.KeyFilePath, "TLS key file")

    disableGzip := flag.Bool("disable-gzip", !config.EnableGzip, "Disable Gzip compression")

    flag.Parse()
    return WebServerConfig{
        Address:         config.Address,
        Port:            *port,
        StaticFilesPath: *staticFilesPath,
        ServerName:      *serverName,

        CertFilePath: *certFile,
        KeyFilePath:  *keyFile,

        EnableGzip: !*disableGzip,
    }
}
