# Tiny Web Server
An extremely simple web server module, for efficiently serving static files over HTTP. TLS and GZip encoding is supported, and configurable through the command line or config files.

## Standalone Server
A standalone server application is provided, allowing you to serve files from a given directory. The default config file path is `/etc/tiny-web-server.conf`, and serves files from `/srv/www`

## Config Files
The ini format is used, all settings are under the `server` section, allowing you to add sections for your own application, each setting is listed below.

| Name          | Type    | Description                      |
| ------------- | ------- | -------------------------------- |
| address       | string  | Address the server is bound to.  |
| port          | int     | Port number used.                |
| static        | string  | Directory path of file served.   |
| name          | string  | Name to identify the server.     |
| cert          | string  | File path of TLS certificate.    |
| key           | string  | File path of TLS private key.    |
| http-to-https | boolean | Redirect http requests to https. |
| gzip          | boolean | Enable GZip encoding.            |

If a certificate and key is provided, then TLS will automatically be enabled.

## Go Module
This server is mainly intended to be embedded. For example, to provide a web interface, or host a WebSocket server.

### API
 - `Listen(Config, http.HandlerFunc) error`
   - Start listening for incoming requests, responding with the handler provided.
 - `Handler(Config) http.HandlerFunc`
   - Create a handler for serving your static files.
 - `DefaultConfig() Config`
   - Provides a `Config` with default settings.
 - `FileConfig(*ini.File, Config) Config`
   - Read a config file, using the settings in the given `Config` as defaults.
 - `CommandLineConfig(Config) Config`
   - Read command line options, using the settings in the given `Config` as defaults.

