package config

import (
	"github.com/spf13/viper"
	"fmt"
)

func init() {
	viper.SetConfigName("conf")
	viper.AddConfigPath("$HOME/.appname")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error conf file: %s \n", err))
	}
}