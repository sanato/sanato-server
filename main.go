package main

import (
	"bufio"
	"code.google.com/p/go.crypto/bcrypt"
	"flag"
	"fmt"
	"github.com/Sirupsen/logrus"
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

const (
	DEFAULT_PORT          = 3000
	DEFAULT_ROOT_DATA_DIR = "."
	DEFAULT_TEMP_DATA_DIR = "."
	DEFAULT_WEB_URL       = "/web/"
	DEFAULT_WEB_DIR       = "."

	DEFAULT_TOKEN_CIPHER_SUITE = "HS256"
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

	enableWeb(router, cfg)

	logrus.Info("SERVER STARTED: Listening on port " + fmt.Sprintf("%d", cfg.Port))
	logrus.Infof("WEB ACCESS at http://localhost:%d%s", cfg.Port, cfg.WebURL)
	logrus.Infof("WEBDAV ACCESS at http://localhost:%d/webdav/", cfg.Port)

	//http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), handlers.CombinedLoggingHandler(os.Stdout, router))
	http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), router)
}
func enableWeb(router *httprouter.Router, cfg *config.Config) {
	router.ServeFiles(filepath.Join(cfg.WebURL, "/", "*filepath"), http.Dir(cfg.WebDir))
}
func createConfigFile(cp *config.ConfigProvider) (*config.Config, error) {
	logrus.Warn("No configuration file found, we are going to create one")
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("In which port the server is going to listen ? (%d) : ", DEFAULT_PORT)
	portText, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	portText = strings.TrimSuffix(portText, "\n")

	fmt.Printf("In which folder do you want to keep the files? (%s) : ", DEFAULT_ROOT_DATA_DIR)
	rootDataDir, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	rootDataDir = strings.TrimSuffix(rootDataDir, "\n")

	fmt.Printf("In which folder do you want to keep temporary data? (%s) : ", DEFAULT_TEMP_DATA_DIR)
	rootTempDir, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	rootTempDir = strings.TrimSuffix(rootTempDir, "\n")

	fmt.Printf("In which URL do you want to access the server? (%s) : ", DEFAULT_WEB_URL)
	webURL, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	webURL = strings.TrimSuffix(webURL, "\n")

	fmt.Printf("In which folder is the web app? (%s) : ", DEFAULT_WEB_DIR)
	webDir, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	webDir = strings.TrimSuffix(webDir, "\n")

	// validate port is a number
	var port uint64
	if portText == "" {
		port = DEFAULT_PORT
	} else {
		port, err = strconv.ParseUint(portText, 10, 64)
		if err != nil {
			return nil, err
		}
	}

	// clean rootDataDir and rootTempDir
	rootDataDir = filepath.Clean(rootDataDir)
	rootTempDir = filepath.Clean(rootTempDir)
	webURL = filepath.Clean(webURL)
	webDir = filepath.Clean(webDir)

	if rootDataDir == "" {
		rootDataDir = DEFAULT_ROOT_DATA_DIR
	}
	if rootTempDir == "" {
		rootTempDir = DEFAULT_TEMP_DATA_DIR
	}
	if webURL == "" || webURL == "." || webURL == "/" {
		webURL = DEFAULT_WEB_URL
	}
	if webDir == "" {
		webDir = DEFAULT_WEB_DIR
	}

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
	newConfig.TokenCipherSuite = DEFAULT_TOKEN_CIPHER_SUITE
	newConfig.WebURL = webURL
	newConfig.WebDir = webDir

	err = cp.CreateNewConfig(newConfig)
	if err != nil {
		return nil, err
	}

	return newConfig, nil
}
func createUser(ap *auth.AuthProvider) error {
	logrus.Warn("No authentication file found, we are going to create one")
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
