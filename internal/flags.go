package internal

import "flag"

var ConfigPath *string
var DatabasePath *string

func ParseFlags() {
	ConfigPath = flag.String("config", "config.toml", "path to config file")
	DatabasePath = flag.String("db", "status.db", "path to database file")
	flag.Parse()
}
