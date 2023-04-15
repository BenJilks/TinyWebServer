module standalone

go 1.20

require (
	github.com/benjilks/tinywebserver v0.0.3
	github.com/sirupsen/logrus v1.9.0
	gopkg.in/ini.v1 v1.67.0
)

require golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect

replace github.com/benjilks/tinywebserver v0.0.3 => ../
