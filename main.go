package main

import (
	"bufio"
	"code.google.com/p/go.crypto/bcrypt"
	"flag"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/jmcvetta/randutil"
	"github.com/julienschmidt/httprouter"
	authz "github.com/sanato/sanato-api/auth"
	"github.com/sanato/sanato-api/files"
	"github.com/sanato/sanato-lib/auth"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	cfg, err := configProvider.Parse()
	if err != nil {
		// we try to create the cfg file
		cfg, err = createConfigFile(configProvider)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
	}
	err = authProvider.ExistsAuth()
	if err != nil {
		// we try to create the auth file with admin user
		err = createUser(authProvider)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
	}
	storageProvider, err := storage.NewStorageProvider(cfg.RootDataDir, cfg.RootTempDir)
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

	logrus.Info("SERVER STARTED: Listening on port " + fmt.Sprintf(":%d", cfg.Port))

	router.ServeFiles("/web/*filepath", http.Dir("../sanato-web"))
	http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), handlers.CombinedLoggingHandler(os.Stdout, router))
}

func createConfigFile(cp *config.ConfigProvider) (*config.Config, error) {
	logrus.Warn("No configuration file found, creating one")
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter port: ")
	portText, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	portText = strings.TrimSuffix(portText, "\n")

	fmt.Print("Enter root data directory: ")
	rootDataDir, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	rootDataDir = strings.TrimSuffix(rootDataDir, "\n")

	fmt.Print("Enter root temporary directory: ")
	rootTempDir, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	rootTempDir = strings.TrimSuffix(rootTempDir, "\n")

	// validate port is a number
	port, err := strconv.ParseUint(portText, 10, 64)
	if err != nil {
		return nil, err
	}

	// clean rootDataDir and rootTempDir
	rootDataDir = filepath.Clean(rootDataDir)
	rootTempDir = filepath.Clean(rootTempDir)

	// create random secret for signing tokens
	secret, err := randutil.AlphaString(20)
	if err != nil {
		return nil, err
	}

	var newConfig = &config.Config{}
	newConfig.Port = int(port)
	newConfig.RootDataDir = rootDataDir
	newConfig.RootTempDir = rootTempDir
	newConfig.TokenSecret = secret
	newConfig.TokenCipherSuite = "HS256"

	err = cp.CreateNewConfig(newConfig)
	if err != nil {
		return nil, err
	}

	return newConfig, nil
}
func createUser(ap *auth.AuthProvider) error {
	logrus.Warn("No authentication file found, creating one")
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	username = strings.TrimSuffix(username, "\n")

	fmt.Print("Enter password: ")
	password, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	password = strings.TrimSuffix(password, "\n")

	fmt.Print("Enter display name: ")
	displayName, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	displayName = strings.TrimSuffix(displayName, "\n")

	fmt.Print("Enter email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	email = strings.TrimSuffix(email, "\n")

	hashedPasswd, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return err
	}

	var user = &auth.User{}
	user.Username = username
	user.Password = string(hashedPasswd)
	user.DisplayName = displayName
	user.Email = email

	return ap.CreateUser(user)
}
