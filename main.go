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
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

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

var assetManifest map[string]string // webpack asset manifest

var (
	title      = "YourPlace"
	version    = "0.1.0" // Triggers a release build
	protocol   = "http"  // http or https
	tlsCert    = host.GetInstallDir() + "server.cert"
	tlsKey     = host.GetInstallDir() + "server.key"
	cryptoSeed = security.RandomBytes(32) // set in 'c' command line flag
	domain     = "localhost"
	port       = 42424
	debug      = false // set in 'd' command line flag or set debug file in data directory
	gateway    = false // set in 'g' command line flag
	patch      = false // set in 'p' command line flag
	ui         = true  // set in 'du' command line flag
	indexer    = true  // set in 'i' command line flag
	shortcut   = false // set in 's' command line flag
)

func main() {
	time.Sleep(3 * time.Second)   // Sleep to allow the previous instance to close
	_ = core.LogInit("yourplace") // Initialize the logger
	core.LogInfo("~~~~~~~~~~~~~ Starting YourPlace " + version + " ~~~~~~~~~~~~~")
	core.LogDebug("Runtime User: " + host.GetUsername())
	core.LogDebug("Install Directory: " + host.GetInstallDir())
	core.LogDebug("Data Directory: " + host.GetDataDir())

	// --- Command Line Arguments --- //
	var hexString string // Crypto seed hex encoded
	debugPtr := flag.Bool("d", false, "Enable Debug mode, default: false")
	gatewayPtr := flag.Bool("g", false, "Enable Gateway mode, default: false")
	disableUIPtr := flag.Bool("du", false, "Disable opening browser UI, default: false (UI enabled)")
	disableIndexerPtr := flag.Bool("di", false, "Disable automatic blockchain indexing, default: false (indexer enabled)")
	patchPtr := flag.Bool("p", false, "Start patching of YourPlace, default: false")
	shortcutPtr := flag.Bool("s", false, "Started server via shortcut, default: false")
	flag.StringVar(&hexString, "c", "", "A 32-byte array represented as a 64-character hexadecimal string used to synchronize the cryptographic state in a distributed deployment, default: random 32-byte value") // go run main.go -c=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
	flag.Parse()
	// Assign parsed flag values to variables
	debug = *debugPtr
	gateway = *gatewayPtr
	ui = !*disableUIPtr           // UI enabled by default, disabled if -du flag is present
	indexer = !*disableIndexerPtr // Indexer enabled by default, disabled if -di flag is present
	patch = *patchPtr
	shortcut = *shortcutPtr

	// --- Environment Variables --- //
	host.SetEnvVar("YourPlaceProtocol", protocol)
	host.SetEnvVar("YourPlacePort", strconv.Itoa(port))
	domainTemp := os.Getenv("YOURPLACE_ORIGIN") // Get origin domain from environment variable (example: app.yourplace.network)
	if domainTemp == "" || domainTemp == "localhost" {
		domain = "localhost"
		host.SetEnvVar("YourPlaceDomain", "localhost")
	} else {
		domain = domainTemp
		host.SetEnvVar("YourPlaceDomain", domainTemp)
	}
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
	database.Init(filepath.Join(host.GetDataDir(), "yourplace.sqlite.db"), "sqlite")
	if !database.Ping() {
		core.LogFatal("Could not connect to database")
	}
	database.SetDefaults()                    // Sets default database entries if not existing
	installed := routes.IsInstalled(database) // checking if the server is installed
	if gateway && !installed {
		core.LogInfo("Gateway mode detected - initializing with default values")
		uploadDir := filepath.Join(host.GetDataDir(), "uploads")
		if !host.DoesExist(uploadDir) {
			host.CreateFolder(uploadDir)
		}
		database.SetGatewayDefaults(uploadDir)
		installed = routes.IsInstalled(database) // Re-check installation status
	}
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
	i := 0
	for {
		i++
		time.Sleep(5 * time.Second)
		if network.IsInternetConnected() {
			break
		} else {
			core.LogDebug("No internet connection - trying again in 5 seconds")
		}
		if i >= 12 {
			break
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
	if gateway {
		_blockchain.InitGateway(database)
	} else {
		_blockchain.Init(database)
	}

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
	StartWebServer(database, _blockchain, ipfs, installed, domain)

	// --- Systray --- //
	runSystray(database)

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
func LoadAssetManifest() {
	manifestData, err := wwwFS.ReadFile("src/www/manifest.json")
	if err != nil {
		core.LogWarn("Could not load asset manifest: " + err.Error())
		assetManifest = make(map[string]string)
		return
	}
	if err := json.Unmarshal(manifestData, &assetManifest); err != nil {
		core.LogWarn("Could not parse asset manifest: " + err.Error())
		assetManifest = make(map[string]string)
	}
}
func LoadTemplates(engine *gin.Engine, embedFS embed.FS, pattern string) {
	// https://github.com/gin-gonic/gin/issues/2795
	root := template.New("")
	engine.SetFuncMap(template.FuncMap{
		"asset": func(name string) string {
			if assetManifest == nil {
				return "/static/js/" + name
			}
			if hashed, ok := assetManifest[name]; ok {
				return hashed
			}
			// Return empty string for split chunks that don't exist in dev builds
			if name == "common.js" || name == "vendors.js" || name == "runtime.js" {
				return ""
			}
			return "/static/js/" + name
		},
	})
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
	// Gateway mode: trigger snapshot catch-up on first run
	if gateway {
		lastCatchUpStr := database.MetaGetValue("indexerCatchUpLastRun")
		if lastCatchUpStr == "" {
			core.LogInfo("Gateway first run detected - triggering snapshot catch-up")
			success, message := blockchain.IndexerCatchUpAll(database, "base")
			if success {
				core.LogInfo("Gateway snapshot catch-up: " + message)
			} else {
				core.LogWarn("Gateway snapshot catch-up failed: " + message)
			}
		}
	}
}
func StartWebServer(database *db.Database, _blockchain *blockchain.Blockchain, ipfs *network.IPFS, installed bool, domain string) {
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
	router.Use(middleware.CORSMiddleware(gateway, domain))
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.LoopbackMiddleware(port, gateway))
	router.Use(middleware.LoopbackRedirectMiddleware(port))
	router.Use(middleware.CSRFMiddleware(middleware.CSRFConfig{CryptoSeed: cryptoSeed}))
	router.Use(middleware.AuthMiddleware(cryptoSeed, database))
	if !installed && !gateway {
		router.Use(middleware.SetupMiddleware(installed))
	}
	//router.Use(middleware.HotPatch())
	//router.Use(middleware.IdsMiddleware())
	router.Use(middleware.RateLimitMiddleware())
	router.Use(middleware.ContentTypeMiddleware())
	router.Use(middleware.CacheControlMiddleware())
	router.Use(middleware.BlockedContent(database))
	router.Use(security.Headers(port))
	LoadAssetManifest()
	LoadTemplates(router, templateFS, "src/templates/*tmpl")
	router.StaticFS("/static", staticFS())
	router.MaxMultipartMemory = 8 << 20
	routes.NotFoundRoutes(router, title, gateway)
	routes.HomeRoutes(router, title, favicon, installed, database, cryptoSeed, gateway)
	routes.FAQRoutes(router, title, database, cryptoSeed, gateway)
	if gateway {
		routes.RPCRoutes(router, database)
	}
	routes.SettingsRoutes(router, title, database, _blockchain, cryptoSeed, gateway, ipfs, debug)
	routes.LoginRoutes(router, title, database, cryptoSeed, domain, port, installed, gateway)
	if !installed {
		routes.SetupRoutes(router, database, title, favicon, port)
	} else {
		routes.ProfileRoutes(router, title, database, _blockchain, gateway)
		routes.PostRoutes(router, database)
		routes.FeedRoutes(router, database)
		routes.FilesRoutes(router, database, ipfs, port, gateway)
		routes.MentalHealthRoutes(router, title, database, cryptoSeed, gateway)
		routes.SearchRoutes(router, database, _blockchain)
		routes.ServicesRoutes(router, database)
		routes.NotificationRoutes(router, database, gateway)
		routes.WalletRoutes(router, title, database, cryptoSeed, gateway)
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
			core.LogInfo("Loading TLS certificate from cert.pem and cert.key")
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				core.LogWarn("Could not load TLS certificate: " + err.Error())
			} else {
				core.LogInfo("Starting TLS server on port 443 for gateway mode")
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
			}
		} else {
			core.LogWarn("Gateway mode TLS certificates not found - TLS server on port 443 disabled")
			core.LogWarn("To enable TLS: place cert.pem and cert.key in " + host.GetDataDir())
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
				blockchain.IndexerStop()
				return
			}
			_ = blockchain.IndexerFetchData(database, _blockchain, "base")
			_ = blockchain.AlgorandIndexerFetchData(database, _blockchain, "algorand")
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
