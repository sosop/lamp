package server

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/spf13/viper"
)

func Monitor() {
	if viper.GetString("mode") == "debug" {
		http.ListenAndServe("0.0.0.0:16666", nil)
	}
}
