//go:build go1.24

package main

import (
	"YourPlace/src/core"
	"YourPlace/src/core/db"
	"YourPlace/src/core/db/blockchain"
	"YourPlace/src/core/host"
	"YourPlace/src/core/middleware"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	"YourPlace/src/core/services"
	"YourPlace/src/routes"
	"crypto/tls"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	_cron "github.com/robfig/cron/v3"
)

//go:embed src/templates
var templateFS embed.FS

//go:embed src/www
var wwwFS embed.FS

//go:embed src/www/image/favicon.ico
var favicon []byte

//go:embed resources/windows/app.manifest
var manifest []byte // embed Windows manifest

var (
	title      = "YourPlace"
	version    = "0.1.0" // Triggers a release build
	protocol   = "http"  // http or https
	tlsCert    = host.GetInstallDir() + "server.cert"
	tlsKey     = host.GetInstallDir() + "server.key"
	cryptoSeed = security.RandomBytes(32)
	domain     = "localhost"
	port       = 42424
	debug      = false // set in 'm' command line flag or set debug file in data directory
	gateway    = false // set in 'g' command line flag
	patch      = false // set in 'p' command line flag
	ui         = true  // set in 'u' command line flag
	indexer    = true  // set in 'i' command line flag
	shortcut   = false // set in 's' command line flag
)

func main() {
	time.Sleep(3 * time.Second)          // Sleep to allow the previous instance to close
	logFile := core.LogInit("yourplace") // Initialize the logger
	core.LogInfo("~~~~~~~~~~~~~ Starting YourPlace " + version + " ~~~~~~~~~~~~~")
	core.LogDebug("Runtime User: " + host.GetUsername())

	// --- Command Line Arguments --- //
	var hexString string // Crypto seed hex encoded
	flag.BoolVar(&debug, "d", false, "Enable Debug mode, default: false")
	flag.BoolVar(&gateway, "g", false, "Enable Gateway mode, default: false")
	flag.BoolVar(&ui, "u", true, "Toggle opening browser UI automatically, default: true")
	flag.BoolVar(&indexer, "i", true, "Toggle automatic blockchain indexing, default: true")
	flag.BoolVar(&patch, "p", false, "Start patching of YourPlace, default: false")
	flag.BoolVar(&shortcut, "s", false, "Started server via shortcut, default: false")
	flag.StringVar(&hexString, "c", "", "A 32-byte array represented as a 64-character hexadecimal string used to synchronize the cryptographic state in a distributed deployment, default: random 32-byte value") // go run main.go -c=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
	flag.Parse()

	// --- Environment Variables --- //
	host.SetEnvVar("YourPlaceProtocol", protocol)
	host.SetEnvVar("YourPlaceDomain", domain)
	host.SetEnvVar("YourPlacePort", strconv.Itoa(port))
	if host.DoesExist(host.GetDataDir() + "noindexer") { // disable indexer via file flag
		indexer = false
	}
	if debug || host.DoesExist(host.GetDataDir()+"debug") { // Run in debug mode
		core.LogInfo("Running in debug mode")
		host.SetEnvVar("YourPlaceDebug", "true")
		debug = true
	} else {
		host.DeleteEnvVar("YourPlaceDebug")
		host.DeleteIfExists(host.GetDataDir() + "debug")
	}
	if gateway { // Run as a gateway
		core.LogInfo("Running as a gateway")
		host.SetEnvVar("YourPlaceGateway", "true")
	} else {
		host.DeleteEnvVar("YourPlaceGateway")
	}
	if patch { // If started in patching mode
		core.LogDebug("Running in patching mode")
		if host.IsAdmin() {
			StartPatching()
		}
		host.Shutdown(0) // Important to exit after patching
	}
	if shortcut { // If started via shortcut
		core.LogDebug("Running via shortcut")
		if !host.CreateMutex("YourPlace") { // if another instance is already running, open the UI and exit
			host.OpenBrowser(protocol + "://" + domain + ":" + strconv.Itoa(port))
			host.Shutdown(0)
		}
	}
	if !host.CreateMutex("YourPlace") { // ensure only one instance is running
		core.LogFatal("Another instance of YourPlace is already running")
	}
	defer host.ReleaseMutex()
	if host.IsAdmin() {
		core.LogWarn("YourPlace is running as an administrator - Not recommended")
	}

	host.DeleteAll(host.GetInstallDir() + "yourplace.version")
	_ = os.WriteFile(host.GetInstallDir()+"yourplace.version", []byte(version), 0644) // write the version string to file

	// --- Pre-Install --- //
	core.LogDebug("Pre-install setup")
	if host.PreInstall(favicon) != true {
		core.LogFatal("Pre-install setup failed")
	}

	// --- Database --- //
	core.LogDebug("Initializing database")
	database := new(db.Database)
	database.Init(host.GetDataDir(), "sqlite")
	if !database.Ping() {
		core.LogFatal("Could not connect to database")
	}
	database.SetDefaults()                    // Sets default database entries if not existing
	installed := routes.IsInstalled(database) // checking if the server is installed
	if installed {
		core.LogDebug("YourPlace is installed at " + host.GetInstallDir())
		core.LogDebug("YourPlace stores your data at " + host.GetDataDir())
	} else {
		core.LogDebug("YourPlace is not installed")
	}

	// --- Dispatch crypto seed --- //
	core.LogDebug("Dispatching crypto seed")
	if security.IsValidCryptoHex(hexString) { // If the provided crypto seed is valid
		decodedString, _ := hex.DecodeString(hexString)
		cryptoSeed = decodedString
	} else { // Else use the provided crypto seed from the database
		cryptoSeedValue := database.SettingsGetValue("cryptoSeed")
		if len(cryptoSeedValue) < 32 {
			database.SettingsUpdateValue("cryptoSeed", string(cryptoSeed))
		} else {
			cryptoSeed = []byte(cryptoSeedValue)
		}
	}

	// --- Network Checking --- //
	core.LogDebug("Checking network")
	for {
		time.Sleep(5 * time.Second)
		if network.IsInternetConnected() {
			break
		} else {
			core.LogDebug("No internet connection - trying again in 5 seconds")
		}
	}
	publicIP, err := network.GetPublicIP()
	if err == nil {
		database.SettingsUpdateValue("publicIP", publicIP.String())
	} else {
		core.LogError("Could not get public IP: " + err.Error())
		database.SettingsUpdateValue("publicIP", "")
	}

	// --- Blockchain --- //
	core.LogDebug("Initializing blockchain")
	_blockchain := new(blockchain.Blockchain)
	_blockchain.Init(database)

	// --- IPFS --- //
	core.LogDebug("Initializing IPFS")
	ipfs := new(network.IPFS)
	ipfs.Init(uint64(port + 1)) // Initialize IPFS daemon listening on YourPlace port + 1
	database.SettingsUpdateValue("ipfsPort", fmt.Sprint(port+1))
	if !ipfs.IPFSNodeAlive() {
		core.LogFatal("IPFS node is not alive")
	}
	publicIP, err = network.GetPublicIP()
	if err != nil {
		core.LogError("Could not get public IP: " + err.Error())
	} else if publicIP != nil {
		if !network.IsTCPPortOpen(publicIP.String(), port+1) {
			core.LogWarn("IPFS port is not open to the internet")
		}
	}

	// --- Start Cron Jobs --- //
	if installed {
		core.LogDebug("Starting cron jobs")
		StartCronJobs(database, _blockchain)
	}

	// --- Start Web Server --- //
	core.LogDebug("Starting web server")
	StartWebServer(database, _blockchain, ipfs, installed, logFile)

	// --- Systray --- //
	systray.Run(func() { SystrayOnReady(database) }, func() { host.Shutdown(0) })

	host.Shutdown(0)
}

// ---------- Startup / Server Functions ---------- //
func staticFS() http.FileSystem {
	// https://github.com/gin-contrib/static/issues/19#issuecomment-963604838
	sub, err := fs.Sub(wwwFS, "src/www")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}
func LoadTemplates(engine *gin.Engine, embedFS embed.FS, pattern string) {
	// https://github.com/gin-gonic/gin/issues/2795
	root := template.New("")
	loadFunc := func(funcMap template.FuncMap, rootTemplate *template.Template, embedFS embed.FS, pattern string) error {
		pattern = strings.ReplaceAll(pattern, ".", "\\.")
		pattern = strings.ReplaceAll(pattern, "*", ".*")
		err := fs.WalkDir(embedFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if matched, _ := regexp.MatchString(pattern, path); !d.IsDir() && matched {
				data, readErr := embedFS.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				t := root.New(path).Funcs(engine.FuncMap)
				if _, parseErr := t.Parse(string(data)); parseErr != nil {
					return parseErr
				}
			}
			return nil
		})
		return err
	}
	tmpl := template.Must(root, loadFunc(engine.FuncMap, root, embedFS, pattern))
	engine.SetHTMLTemplate(tmpl)
}
func PostServerRun(database *db.Database) {
	// Functions that run after the YourPlace server listener starts
	time.Sleep(2 * time.Second) // Sleep for 2 seconds to allow the server to start
	yourPlaceURL := protocol + "://" + domain + ":" + strconv.Itoa(port) + "/"
	core.LogInfo("YourPlace URL: " + yourPlaceURL) // Print YourPlace URL
	if ui {
		//core.LogDebug("Opening browser here normally, but not, to test the cmd.exe spamming issue")
		host.OpenBrowser(protocol + "://" + domain + ":" + strconv.Itoa(port))
	}
	publicIP := database.MetaGetValue("publicIP") // Check if YourPlace port is open to the internet
	if !network.IsTCPPortOpen(publicIP, port) {
		core.LogWarn("YourPlace port " + strconv.Itoa(port) + " is not open to the internet")
		database.MetaUpdateValue("ypPortOpen", "false")
	} else {
		database.MetaUpdateValue("ypPortOpen", "true")
	}
}
func StartWebServer(database *db.Database, _blockchain *blockchain.Blockchain, ipfs *network.IPFS, installed bool, logFile *os.File) {
	if debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.RedirectTrailingSlash = true
	if gateway {
		router.TrustedPlatform = gin.PlatformCloudflare
	} else {
		_ = router.SetTrustedProxies(nil)
	}
	//router.Use(gin.Logger()) // Attach default logger which prints to stdout
	router.Use(CustomGinRecovery())
	router.Use(middleware.CORSMiddleware())
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.LoopbackMiddleware(port))
	router.Use(middleware.LoopbackRedirectMiddleware(port))
	router.Use(middleware.GatewayMiddleware(gateway))
	router.Use(middleware.CSRFMiddleware(middleware.CSRFConfig{CryptoSeed: cryptoSeed}))
	router.Use(middleware.AuthMiddleware(cryptoSeed, database))
	if !installed {
		router.Use(middleware.SetupMiddleware(installed))
	}
	router.Use(middleware.HotPatch())
	router.Use(middleware.IdsMiddleware())
	router.Use(middleware.RateLimitMiddleware())
	router.Use(middleware.ContentTypeMiddleware())
	router.Use(middleware.CacheControlMiddleware())
	router.Use(middleware.BlockedContent(database))
	router.Use(security.Headers(port))
	LoadTemplates(router, templateFS, "src/templates/*tmpl")
	router.StaticFS("/static", staticFS())
	router.MaxMultipartMemory = 8 << 20
	routes.NotFoundRoutes(router, title, gateway)
	routes.HomeRoutes(router, title, favicon, installed, database, cryptoSeed, gateway)
	routes.FAQRoutes(router, title, database, cryptoSeed, gateway)
	routes.SettingsRoutes(router, title, database, _blockchain, cryptoSeed, gateway, ipfs, debug)
	routes.LoginRoutes(router, title, database, cryptoSeed, domain, installed, gateway)
	if !installed {
		routes.SetupRoutes(router, database, title, favicon, port)
	} else {
		routes.ProfileRoutes(router, title, database, _blockchain, cryptoSeed, gateway)
		routes.PostRoutes(router, database)
		routes.FeedRoutes(router, database)
		routes.FilesRoutes(router, database, ipfs, port)
		routes.MentalHealthRoutes(router, title, database, cryptoSeed, gateway)
		routes.SearchRoutes(router, database, _blockchain)
		routes.ServicesRoutes(router, database)
		routes.NotificationRoutes(router, database)
		if debug {
			routes.TestRoutes(router, title, gateway)
		}
	}
	// --- Start Web Server Loop --- //
	var srv *http.Server
	srv = &http.Server{
		Addr:              "127.0.0.1:" + strconv.Itoa(port),
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 20 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB max header size
	}
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			core.LogFatal("Could not start server: " + err.Error())
		}
	}()

	if gateway { // Gateway mode TLS server
		certPath := host.GetDataDir() + "cert.pem"
		keyPath := host.GetDataDir() + "cert.key"
		if host.DoesExist(certPath) && host.DoesExist(keyPath) {
			core.LogInfo("Starting TLS server on port 443 for gateway mode")
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				core.LogError("Could not load TLS certificate: " + err.Error())
			}
			tlsConfig := &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}
			tlsSrv := &http.Server{
				Addr:              "0.0.0.0:443",
				Handler:           router,
				ReadTimeout:       30 * time.Second,
				WriteTimeout:      60 * time.Second,
				IdleTimeout:       120 * time.Second,
				ReadHeaderTimeout: 20 * time.Second,
				MaxHeaderBytes:    1 << 20, // 1 MB max header size
				TLSConfig:         tlsConfig,
			}
			go func() {
				err = tlsSrv.ListenAndServeTLS("", "")
				if err != nil {
					core.LogError("Could not start TLS server: " + err.Error())
				}
			}()
		} else {
			core.LogError("Gateway mode enabled but cert.pem or cert.key not found in data directory")
		}
	}

	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if network.IsTCPPortOpen("127.0.0.1", port) {
			break
		}
	}
	PostServerRun(database)
}
func StartCronJobs(database *db.Database, _blockchain *blockchain.Blockchain) {
	// --- Scheduled Jobs --- //
	c := _cron.New(_cron.WithSeconds())
	// ------- ETH Price Updater ------- //
	ethPriceUSD, err := services.CoinbaseGetPriceUSD("ETH")
	if err == nil && ethPriceUSD != 0 {
		database.MetaUpdateValue("ethPriceUSD", fmt.Sprint(ethPriceUSD))
	}
	c.AddFunc("@every 10m", func() { // ETH price updater
		ethPriceUSD, err = services.CoinbaseGetPriceUSD("ETH")
		if err == nil && ethPriceUSD != 0 {
			database.MetaUpdateValue("ethPriceUSD", fmt.Sprint(ethPriceUSD))
		}
	})
	// ------- Expire Old Auth Material ------- //
	database.AuthExpireLoginNonce()
	c.AddFunc("@every 1m", func() {
		database.AuthExpireLoginNonce()
	})
	// ------- Blockchain Indexer ------- //
	if indexer {
		blockchain.IndexerClearOldCachedPosts(database)
		c.AddFunc("@every 60m", func() { // clean out the cached posts
			blockchain.IndexerClearOldCachedPosts(database)
		})
		blockchain.IndexerRestartJobs(database, "base") // set any jobs to "failed" that were left hanging on startup
		c.AddFunc("@every 2m", func() {
			indexerOnBattery := database.SettingsGetValue("indexerOnBattery")
			indexerOnBatteryBool, _ := strconv.ParseBool(indexerOnBattery)
			isOnBattery := host.IsOnBattery()
			if isOnBattery && !indexerOnBatteryBool { // Don't run the indexer if the computer is on battery
				core.LogDebug("Host is on battery - skipping indexer run")
				blockchain.IndexerStop()
				return
			}
			core.LogDebug("Starting Base Indexer Run")
			blockchain.IndexerFetchData(database, _blockchain, "base")
		})
	}
	// ------- IPFS BadBits ------- //
	if !host.DoesExist(host.GetDataDir() + ".ipfs" + host.PathSeparator + "denylists" + host.PathSeparator + "badbits.deny") {
		core.LogDebug("Badbits doesn't exist - creating")
		network.UpdateBadBits(database)
	}
	c.AddFunc("@every 168h", func() {
		network.UpdateBadBits(database)
	})
	// ------- Clear Caches ------- //
	c.AddFunc("@every 10m", func() {
		db.CleanAllCaches()
	})
	// --- Start Cron --- //
	c.Start()
}
func StartPatching() {
	helperServiceName := "YourPlaceHelper"
	host.RemoveScheduledTask(helperServiceName)
	host.InstallHelper()
}
func CustomGinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			err := recover()
			if err != nil {
				stack := make([]byte, 4096)
				stack = stack[:runtime.Stack(stack, false)]
				core.LogError(fmt.Sprintf("PANIC RECOVERED: %v\n%s", err, stack))
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

// --- Systray Functions --- //
func SystrayOnReady(database *db.Database) {
	systray.SetTemplateIcon(favicon, favicon)
	if runtime.GOOS == "windows" {
		systray.SetIcon(favicon)
		systray.SetTitle("YourPlace")
	}
	systray.SetTooltip("YourPlace Server")
	mUI := systray.AddMenuItem("Open YourPlace", "Open YourPlace in your browser")
	mUI.SetIcon(favicon)
	mSettings := systray.AddMenuItem("Settings", "YourPlace Settings")
	mQuit := systray.AddMenuItem("Quit", "Quit YourPlace Server")
	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				host.Shutdown(0)
			case <-mUI.ClickedCh:
				host.OpenBrowser(protocol + "://" + domain + ":" + strconv.Itoa(port))
			case <-mSettings.ClickedCh:
				host.OpenBrowser(protocol + "://" + domain + ":" + strconv.Itoa(port) + "/settings")
			}
		}
	}()
}
