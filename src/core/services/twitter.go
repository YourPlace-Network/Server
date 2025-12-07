package services

import (
	"YourPlace/src/core"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/dghubble/oauth1"
)

type Post struct {
	Username string
	Content  string
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.3"

var typingSpeed = 150 * time.Millisecond // How much delay between each keystroke to simulate human input (slow typing adds a random +-20% variation to this)
var chromeOptions = append(chromedp.DefaultExecAllocatorOptions[:],
	chromedp.Flag("headless", false), // todo debug
	chromedp.UserAgent(userAgent),
	chromedp.Flag("disable-popup-blocking", true),
)

func TwitterTest() {
	email := host.GetEnvVar("X_EMAIL")
	username := host.GetEnvVar("X_USERNAME")
	password := host.GetEnvVar("X_PASSWORD")
	cookies, __error := LogInToTwitter(email, username, password)
	if __error != nil {
		core.LogError("Could not log into x.com: " + __error.Error())
	} else {
		core.LogInfo("Logged into x.com")
		for cookie := range cookies {
			core.LogInfo("\tCookie: " + cookies[cookie].Name)
			core.LogInfo("\tValue: " + cookies[cookie].Value)
		}
	}
	host.Shutdown(0)
}
func LogInToTwitter(email, username, password string) ([]*network.Cookie, error) {
	core.LogDebug("Logging into x.com")

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), chromeOptions...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, timeoutCancel := context.WithTimeout(ctx, 240*time.Second)
	defer timeoutCancel()

	captureHTML := func(stepName string) {
		var html string
		err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(`document.documentElement.outerHTML`, &html))
		if err == nil {
			filename := fmt.Sprintf("%sdebug_%s.html", host.GetDataDir(), stepName)
			err = os.WriteFile(filename, []byte(html), 0644)
			if err == nil {
				core.LogDebug(fmt.Sprintf("HTML saved to %s", filename))
			} else {
				core.LogDebug(fmt.Sprintf("Failed to save HTML: %s", err.Error()))
			}
		} else {
			core.LogDebug(fmt.Sprintf("Failed to capture HTML: %s", err.Error()))
		}
	}
	captureScreenshot := func(stepName string) {
		var buf []byte
		err := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 90))
		if err == nil {
			filename := fmt.Sprintf("%sscreenshot_%s.png", host.GetDataDir(), stepName)
			err = os.WriteFile(filename, buf, 0644)
			if err == nil {
				core.LogDebug(fmt.Sprintf("Screenshot saved to %s", filename))
			} else {
				core.LogDebug(fmt.Sprintf("Failed to save screenshot: %s", err.Error()))
			}
		} else {
			core.LogDebug(fmt.Sprintf("Failed to capture screenshot: %s", err.Error()))
		}
	}
	capturePage := func(stepName string) {
		captureHTML(stepName)
		captureScreenshot(stepName)
	}
	_ = capturePage // todo debug

	// Navigate to the login page
	core.LogDebug("Navigating to Twitter")
	err := chromedp.Run(ctx,
		chromedp.Navigate("https://x.com/i/flow/login"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(time.Duration(security.RandomUint(2, 4))*time.Second),
	)
	if err != nil {
		return nil, core.LogErrorReturn("Login Page Navigation Error: " + err.Error())
	}

	// Enter email address
	core.LogDebug("Email Entry")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible("input[autocomplete=\"username\"]", chromedp.ByQuery),
		slowType("input[autocomplete=\"username\"]", email+kb.Enter),
		chromedp.Sleep(time.Duration(security.RandomUint(5, 10))*time.Second),
	)
	if err != nil {
		return nil, core.LogErrorReturn("Username Entry Error: " + err.Error())
	}

	// Check if "Enter your phone number or username" prompt appears
	core.LogDebug("Checking if phone number or username is required")
	var pageText string
	err = chromedp.Run(ctx, chromedp.EvaluateAsDevTools(`document.body.innerText`, &pageText))
	if strings.Contains(pageText, "Enter your phone number or username") {
		core.LogDebug("Username Entry")
		err2 := chromedp.Run(ctx,
			chromedp.WaitVisible("input[data-testid=\"ocfEnterTextTextInput\"]"),
			slowType("input[data-testid=\"ocfEnterTextTextInput\"]", username+kb.Enter),
			chromedp.Sleep(time.Duration(security.RandomUint(5, 10))*time.Second),
		)
		if err2 != nil {
			return nil, core.LogErrorReturn("Email Entry Error: " + err2.Error())
		}
	}

	// Enter password
	err = chromedp.Run(ctx,
		chromedp.WaitVisible("input[type=\"password\"]", chromedp.ByQuery),
		slowType("input[type=\"password\"]", password+kb.Enter),
		chromedp.Sleep(time.Duration(security.RandomUint(15, 30))*time.Second),
	)
	if err != nil {
		return nil, core.LogErrorReturn("Password Entry Error: " + err.Error())
	}

	// Check if logged in
	core.LogDebug("Checking if logged in")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible("a[href=\"/compose/post\"]", chromedp.ByQuery),
	)

	// Retrieve all cookies after successful login
	var cookies []*network.Cookie
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookiesResult, err3 := network.GetCookies().Do(ctx)
			if err3 != nil {
				return err3
			}
			cookies = cookiesResult
			return nil
		}),
	)
	if err != nil {
		return nil, core.LogErrorReturn("Cookie Retrieval Error: " + err.Error())
	}
	return cookies, nil
}
func SaveCookies(cookies []*network.Cookie) {
	// todo
	// auth_token = primary authentication cookie for x.com
	// ct0 = CSRF token for authenticated requests
	// guest_id = guest token for unauthenticated requests
	//security.AddSecret("x.com", "cookies")
}
func GetCookies() []*network.Cookie {
	//security.GetSecret("x.com")
	return nil
}
func CheckTwitterCookiesAlive() {
	// todo check if the auth cookies are still alive
}
func RefreshTwitterCookies() {
	// perform a request to /home to refresh the cookie expiration
}
func XcomTestCredentials(apiKey, apiSecret, accessToken, accessTokenSecret string) bool {
	if apiKey == "" || apiSecret == "" || accessToken == "" || accessTokenSecret == "" {
		return false
	}
	oauth1Config := oauth1.NewConfig(apiKey, apiSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	httpClient := oauth1Config.Client(oauth1.NoContext, token)
	httpClient.Timeout = 10 * time.Second
	resp, err := httpClient.Get("https://api.twitter.com/2/users/me")
	if err != nil {
		core.LogDebug("X.com API test failed: " + err.Error())
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		core.LogDebug("X.com API test succeeded")
		return true
	}
	core.LogDebug("X.com API test failed with status: " + resp.Status)
	return false
}
func XcomCreatePost(apiKey, apiSecret, accessToken, accessTokenSecret, text string) bool {
	if apiKey == "" || apiSecret == "" || accessToken == "" || accessTokenSecret == "" || text == "" {
		return false
	}
	oauth1Config := oauth1.NewConfig(apiKey, apiSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	httpClient := oauth1Config.Client(oauth1.NoContext, token)
	httpClient.Timeout = 30 * time.Second
	payload := fmt.Sprintf(`{"text": %q}`, text)
	req, err := http.NewRequest("POST", "https://api.twitter.com/2/tweets", strings.NewReader(payload))
	if err != nil {
		core.LogDebug("X.com create post request failed: " + err.Error())
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		core.LogDebug("X.com create post failed: " + err.Error())
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == 201 {
		core.LogDebug("X.com post created successfully")
		return true
	}
	core.LogDebug("X.com create post failed with status: " + resp.Status)
	return false
}

func slowType(selector, text string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		for _, char := range text {
			err := chromedp.SendKeys(selector, string(char), chromedp.ByQuery).Do(ctx)
			if err != nil {
				return err
			}
			typingSpeedVariable := addTimeVariation(typingSpeed)
			time.Sleep(typingSpeedVariable)
		}
		return nil
	})
}
func addTimeVariation(d time.Duration) time.Duration {
	variation := (rand.Float64() - 0.5) * 0.4
	return d + time.Duration(float64(d)*variation)
}

// --------------------- //
/*
package main

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
    "time"
)

type LocalXClient struct {
    clientID     string
    accessToken  string
    refreshToken string
    httpClient   *http.Client
    tokenFile    string
}

type StoredTokens struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresAt    int64  `json:"expires_at"`
}

// NewLocalXClient creates a client for local device automation
func NewLocalXClient(clientID string) *LocalXClient {
    homeDir, _ := os.UserHomeDir()
    tokenFile := filepath.Join(homeDir, ".yourplace", "x_tokens.json")

    return &LocalXClient{
        clientID:   clientID,
        httpClient: &http.Client{Timeout: 30 * time.Second},
        tokenFile:  tokenFile,
    }
}

// LoadStoredTokens loads tokens from local storage
func (c *LocalXClient) LoadStoredTokens() error {
    data, err := os.ReadFile(c.tokenFile)
    if err != nil {
        return err
    }

    var tokens StoredTokens
    err = json.Unmarshal(data, &tokens)
    if err != nil {
        return err
    }

    c.accessToken = tokens.AccessToken
    c.refreshToken = tokens.RefreshToken

    // Check if token is expired
    if time.Now().Unix() > tokens.ExpiresAt {
        return c.RefreshAccessToken()
    }

    return nil
}

// SaveTokens securely stores tokens on the local device
func (c *LocalXClient) SaveTokens(expiresIn int) error {
    dir := filepath.Dir(c.tokenFile)
    err := os.MkdirAll(dir, 0700)
    if err != nil {
        return err
    }

    tokens := StoredTokens{
        AccessToken:  c.accessToken,
        RefreshToken: c.refreshToken,
        ExpiresAt:    time.Now().Unix() + int64(expiresIn),
    }

    data, err := json.MarshalIndent(tokens, "", "  ")
    if err != nil {
        return err
    }

    return os.WriteFile(c.tokenFile, data, 0600)
}

// AuthenticateWithBrowser starts local server and opens browser for auth
func (c *LocalXClient) AuthenticateWithBrowser() error {
    // Generate PKCE parameters
    verifier := generateCodeVerifier()
    challenge := generateCodeChallenge(verifier)
    state := generateRandomString(32)

    // Start local server to receive callback
    listener, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        return err
    }
    port := listener.Addr().(*net.TCPAddr).Port
    redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

    // Channel to receive auth code
    codeChan := make(chan string)
    errorChan := make(chan error)

    // Setup HTTP server
    mux := http.NewServeMux()
    mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
        code := r.URL.Query().Get("code")
        returnedState := r.URL.Query().Get("state")

        if returnedState != state {
            errorChan <- fmt.Errorf("state mismatch")
            return
        }

        if code == "" {
            errorChan <- fmt.Errorf("no authorization code received")
            return
        }

        html := `
        <html>
            <body>
                <h2>Authorization successful!</h2>
                <p>You can close this window and return to the application.</p>
                <script>window.close();</script>
            </body>
        </html>`
        w.Write([]byte(html))

        codeChan <- code
    })

    server := &http.Server{Handler: mux}
    go server.Serve(listener)
    defer server.Close()

    // Build authorization URL
    params := url.Values{}
    params.Add("response_type", "code")
    params.Add("client_id", c.clientID)
    params.Add("redirect_uri", redirectURI)
    params.Add("scope", "tweet.read users.read tweet.write offline.access")
    params.Add("state", state)
    params.Add("code_challenge", challenge)
    params.Add("code_challenge_method", "S256")

    authURL := fmt.Sprintf("https://twitter.com/i/oauth2/authorize?%s", params.Encode())

    // Open browser
    fmt.Println("Opening browser for authentication...")
    openBrowser(authURL)

    // Wait for callback
    select {
    case code := <-codeChan:
        // Exchange code for tokens
        return c.exchangeCodeForTokens(code, verifier, redirectURI)
    case err := <-errorChan:
        return err
    case <-time.After(5 * time.Minute):
        return fmt.Errorf("authentication timeout")
    }
}

// exchangeCodeForTokens exchanges auth code for tokens (no client secret needed)
func (c *LocalXClient) exchangeCodeForTokens(code, verifier, redirectURI string) error {
    data := url.Values{}
    data.Set("code", code)
    data.Set("grant_type", "authorization_code")
    data.Set("client_id", c.clientID)
    data.Set("redirect_uri", redirectURI)
    data.Set("code_verifier", verifier)

    req, err := http.NewRequest("POST", "https://api.twitter.com/2/oauth2/token", strings.NewReader(data.Encode()))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("token exchange failed: %s", string(body))
    }

    var tokenResp struct {
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
        ExpiresIn    int    `json:"expires_in"`
    }

    err = json.NewDecoder(resp.Body).Decode(&tokenResp)
    if err != nil {
        return err
    }

    c.accessToken = tokenResp.AccessToken
    c.refreshToken = tokenResp.RefreshToken

    // Save tokens locally
    return c.SaveTokens(tokenResp.ExpiresIn)
}

// RefreshAccessToken refreshes token without client secret
func (c *LocalXClient) RefreshAccessToken() error {
    data := url.Values{}
    data.Set("refresh_token", c.refreshToken)
    data.Set("grant_type", "refresh_token")
    data.Set("client_id", c.clientID)

    req, err := http.NewRequest("POST", "https://api.twitter.com/2/oauth2/token", strings.NewReader(data.Encode()))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("token refresh failed: %s", string(body))
    }

    var tokenResp struct {
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
        ExpiresIn    int    `json:"expires_in"`
    }

    err = json.NewDecoder(resp.Body).Decode(&tokenResp)
    if err != nil {
        return err
    }

    c.accessToken = tokenResp.AccessToken
    if tokenResp.RefreshToken != "" {
        c.refreshToken = tokenResp.RefreshToken
    }

    return c.SaveTokens(tokenResp.ExpiresIn)
}

// PostTweet posts a tweet
func (c *LocalXClient) PostTweet(text string) error {
    payload := map[string]interface{}{
        "text": text,
    }

    jsonData, err := json.Marshal(payload)
    if err != nil {
        return err
    }

    req, err := http.NewRequest("POST", "https://api.twitter.com/2/tweets", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }

    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to post tweet: %s", string(body))
    }

    return nil
}

// Helper functions
func generateCodeVerifier() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.RawURLEncoding.EncodeToString(b)
}

func generateCodeChallenge(verifier string) string {
    hash := sha256.Sum256([]byte(verifier))
    return base64.RawURLEncoding.EncodeToString(hash[:])
}

func generateRandomString(length int) string {
    b := make([]byte, length)
    rand.Read(b)
    return base64.RawURLEncoding.EncodeToString(b)[:length]
}

func openBrowser(url string) {
    var err error
    switch runtime.GOOS {
    case "linux":
        err = exec.Command("xdg-open", url).Start()
    case "windows":
        err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
    case "darwin":
        err = exec.Command("open", url).Start()
    }
    if err != nil {
        fmt.Printf("Please open this URL in your browser:\n%s\n", url)
    }
}

func main() {
    // Public client ID (safe to embed in distributed software)
    client := NewLocalXClient("YOUR_PUBLIC_CLIENT_ID")

    // Try to load existing tokens
    err := client.LoadStoredTokens()
    if err != nil {
        fmt.Println("No valid tokens found. Starting authentication...")
        err = client.AuthenticateWithBrowser()
        if err != nil {
            fmt.Printf("Authentication failed: %v\n", err)
            return
        }
        fmt.Println("Authentication successful!")
    }

    // Now the user's device can perform automated tasks
    err = client.PostTweet("Automated post from my local device!")
    if err != nil {
        fmt.Printf("Error posting: %v\n", err)
        return
    }

    fmt.Println("Tweet posted successfully!")
}




package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
)

// CreatePost creates a new X post with optional media
func (c *LocalXClient) CreatePost(text string) (string, error) {
    payload := map[string]interface{}{
        "text": text,
    }

    jsonData, err := json.Marshal(payload)
    if err != nil {
        return "", err
    }

    req, err := http.NewRequest("POST", "https://api.twitter.com/2/tweets", bytes.NewBuffer(jsonData))
    if err != nil {
        return "", err
    }

    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        err = c.RefreshAccessToken()
        if err != nil {
            return "", err
        }

        // Retry with new token
        req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
        resp, err = c.httpClient.Do(req)
        if err != nil {
            return "", err
        }
        defer resp.Body.Close()
    }

    if resp.StatusCode != http.StatusCreated {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("failed to create post: %s", string(body))
    }

    var result struct {
        Data struct {
            ID   string `json:"id"`
            Text string `json:"text"`
        } `json:"data"`
    }

    err = json.NewDecoder(resp.Body).Decode(&result)
    if err != nil {
        return "", err
    }

    return result.Data.ID, nil
}

// GetUserPosts fetches posts from a specific user's profile
func (c *LocalXClient) GetUserPosts(username string, maxResults int) ([]Tweet, error) {
    // First, get user ID from username
    userEndpoint := fmt.Sprintf("https://api.twitter.com/2/users/by/username/%s", username)

    req, err := http.NewRequest("GET", userEndpoint, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        err = c.RefreshAccessToken()
        if err != nil {
            return nil, err
        }

        req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
        resp, err = c.httpClient.Do(req)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()
    }

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to get user: %s", string(body))
    }

    var userResp struct {
        Data struct {
            ID       string `json:"id"`
            Username string `json:"username"`
            Name     string `json:"name"`
        } `json:"data"`
    }

    err = json.NewDecoder(resp.Body).Decode(&userResp)
    if err != nil {
        return nil, err
    }

    // Now get the user's tweets
    params := url.Values{}
    params.Add("max_results", fmt.Sprintf("%d", maxResults))
    params.Add("tweet.fields", "created_at,public_metrics,author_id")
    params.Add("exclude", "retweets,replies")  // Remove this line to include retweets and replies

    tweetsEndpoint := fmt.Sprintf("https://api.twitter.com/2/users/%s/tweets?%s", userResp.Data.ID, params.Encode())

    req, err = http.NewRequest("GET", tweetsEndpoint, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))

    resp, err = c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to get tweets: %s", string(body))
    }

    var tweetsResp struct {
        Data []Tweet `json:"data"`
        Meta struct {
            NextToken   string `json:"next_token"`
            ResultCount int    `json:"result_count"`
        } `json:"meta"`
    }

    err = json.NewDecoder(resp.Body).Decode(&tweetsResp)
    if err != nil {
        return nil, err
    }

    return tweetsResp.Data, nil
}

// GetHomeTimeline gets the authenticated user's home timeline (includes For You content)
// Note: X API v2 doesn't separate FYP and Following - this returns the combined home feed
func (c *LocalXClient) GetHomeTimeline(maxResults int) ([]Tweet, error) {
    params := url.Values{}
    params.Add("max_results", fmt.Sprintf("%d", maxResults))
    params.Add("tweet.fields", "created_at,public_metrics,author_id")
    params.Add("expansions", "author_id")
    params.Add("user.fields", "username,name,verified")

    endpoint := fmt.Sprintf("https://api.twitter.com/2/users/me/timelines/reverse_chronological?%s", params.Encode())

    req, err := http.NewRequest("GET", endpoint, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        err = c.RefreshAccessToken()
        if err != nil {
            return nil, err
        }

        req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
        resp, err = c.httpClient.Do(req)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()
    }

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to get timeline: %s", string(body))
    }

    var timelineResp struct {
        Data []Tweet `json:"data"`
        Includes struct {
            Users []struct {
                ID       string `json:"id"`
                Name     string `json:"name"`
                Username string `json:"username"`
            } `json:"users"`
        } `json:"includes"`
        Meta struct {
            NextToken   string `json:"next_token"`
            ResultCount int    `json:"result_count"`
        } `json:"meta"`
    }

    err = json.NewDecoder(resp.Body).Decode(&timelineResp)
    if err != nil {
        return nil, err
    }

    return timelineResp.Data, nil
}

// Enhanced Tweet struct with more fields
type Tweet struct {
    ID        string    `json:"id"`
    Text      string    `json:"text"`
    AuthorID  string    `json:"author_id"`
    CreatedAt string    `json:"created_at"`
    Metrics   *Metrics  `json:"public_metrics,omitempty"`
}

type Metrics struct {
    RetweetCount int `json:"retweet_count"`
    ReplyCount   int `json:"reply_count"`
    LikeCount    int `json:"like_count"`
    QuoteCount   int `json:"quote_count"`
}

// Example usage
func main() {
    client := NewLocalXClient("YOUR_PUBLIC_CLIENT_ID")

    // Load or authenticate
    err := client.LoadStoredTokens()
    if err != nil {
        fmt.Println("Authenticating...")
        err = client.AuthenticateWithBrowser()
        if err != nil {
            fmt.Printf("Auth failed: %v\n", err)
            return
        }
    }

    // 1. Create a post
    postID, err := client.CreatePost("Hello from my automated X client!")
    if err != nil {
        fmt.Printf("Error creating post: %v\n", err)
    } else {
        fmt.Printf("Created post with ID: %s\n", postID)
    }

    // 2. Read posts from a specific user's profile
    tweets, err := client.GetUserPosts("elonmusk", 5)
    if err != nil {
        fmt.Printf("Error getting user posts: %v\n", err)
    } else {
        fmt.Println("\n@elonmusk's recent posts:")
        for _, tweet := range tweets {
            fmt.Printf("- %s\n", tweet.Text)
            if tweet.Metrics != nil {
                fmt.Printf("  Likes: %d, Retweets: %d\n", tweet.Metrics.LikeCount, tweet.Metrics.RetweetCount)
            }
        }
    }

    // 3. Get home timeline (includes For You content)
    timeline, err := client.GetHomeTimeline(10)
    if err != nil {
        fmt.Printf("Error getting timeline: %v\n", err)
    } else {
        fmt.Println("\nYour Timeline:")
        for _, tweet := range timeline {
            fmt.Printf("- %s\n", tweet.Text[:min(100, len(tweet.Text))])
        }
    }
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
*/
