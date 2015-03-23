package main

import (
	"flag"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/julienschmidt/httprouter"
	"github.com/whitedav/api/webdav"
	"net/http"
	"os"

	"github.com/whitedav/lib/whitedav/auth"
	"github.com/whitedav/lib/whitedav/config"
	"github.com/whitedav/lib/whitedav/storage"
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

	webdavAPI, err := webdav.NewAPI(router, configProvider, authProvider, storageProvider)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	webdavAPI.Start()

	http.ListenAndServe(":3000", handlers.CombinedLoggingHandler(os.Stdout, router))
}
