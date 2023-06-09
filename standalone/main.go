package main

import (
	"flag"
	"github.com/benjilks/tinywebserver"
	log "github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

type Config struct {
	LogLevel        string
	WebServerConfig webserver.Config
}

func defaultConfig() Config {
	return Config{
		LogLevel:        "info",
		WebServerConfig: webserver.DefaultConfig(),
	}
}

func setLogLevel(levelName string) {
	switch levelName {
	case "panic":
		log.SetLevel(log.PanicLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "trace":
		log.SetLevel(log.TraceLevel)
	default:
		log.SetLevel(log.InfoLevel)
		log.WithField("level", levelName).
			Warn("Unknown log level name")
		levelName = "info"
	}

	log.WithField("log-level", levelName).
		Info("Using log level")
}

func fileConfig(filePath string, config Config) Config {
	configFile, err := ini.Load(filePath)
	if err != nil {
		return config
	}

	logSection := configFile.Section("log")
	return Config{
		LogLevel:        logSection.Key("level").MustString(config.LogLevel),
		WebServerConfig: webserver.FileConfig(configFile, config.WebServerConfig),
	}
}

func commandLineConfig(config Config) Config {
	logLevel := flag.String("log-level", config.LogLevel,
		"Log level (panic, fatal, error, warn, info, debug and trace)")
	configFile := flag.String("config", "", "Config file path")
	webServerConfig := webserver.CommandLineConfig(config.WebServerConfig)

	config = Config{
		LogLevel:        *logLevel,
		WebServerConfig: webServerConfig,
	}

	if *configFile != "" {
		config = fileConfig(*configFile, config)
	}

	return config
}

func main() {
	config := defaultConfig()
	config = fileConfig("/etc/tiny-web-server.conf", config)
	config = commandLineConfig(config)
	setLogLevel(config.LogLevel)

	webHandler := webserver.Handler(config.WebServerConfig)
	if err := webserver.Listen(config.WebServerConfig, webHandler); err != nil {
		log.Fatal(err)
	}
}
