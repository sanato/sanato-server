package main

import (
	"flag"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/julienschmidt/httprouter"
	authz "github.com/sanato/sanato-api/auth"
	"github.com/sanato/sanato-api/files"
	"github.com/sanato/sanato-lib/auth"
	"net/http"
	"os"

	"github.com/sanato/sanato-api/webdav"
	"github.com/sanato/sanato-lib/config"
	"github.com/sanato/sanato-lib/storage"
)

func main() {
	var configFile string
	var authFile string
	flag.StringVar(&configFile, "config", "./config.json", "config=/etc/config.json")
	flag.StringVar(&authFile, "auth", "./auth.json", "auth=/etc/config.json")

	authProvider, err := auth.NewAuthProvider(authFile)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	configProvider, err := config.NewConfigProvider(configFile)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	config, err := configProvider.Parse()
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	storageProvider, err := storage.NewStorageProvider(config.RootDataDir)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	router := httprouter.New()

	authAPI, err := authz.NewAPI(router, configProvider, authProvider, storageProvider)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	webdavAPI, err := webdav.NewAPI(router, configProvider, authProvider, storageProvider)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	filesAPI, err := files.NewAPI(router, configProvider, authProvider, storageProvider)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	authAPI.Start()
	webdavAPI.Start()
	filesAPI.Start()

	router.ServeFiles("/web/*filepath", http.Dir("../sanato-web"))
	http.ListenAndServe(fmt.Sprintf(":%d", config.Port), handlers.CombinedLoggingHandler(os.Stdout, router))
}
