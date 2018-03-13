package server

import (
	_ "net/http/pprof"
	"net/http"
	"github.com/spf13/viper"
)

func Monitor() {
	if viper.GetString("mode") == "debug" {
		http.ListenAndServe("0.0.0.0:16666", nil)
	}
}
