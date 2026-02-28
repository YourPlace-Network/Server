package services

import (
	"YourPlace/src/core"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"context"
	"encoding/json"
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
type XcomPost struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	AuthorID  string `json:"author_id"`
	CreatedAt string `json:"created_at"`
	Username  string `json:"username"`
	Name      string `json:"name"`
}
type XcomUserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"

var typingSpeed = 150 * time.Millisecond // How much delay between each keystroke to simulate human input (slow typing adds a random +-20% variation to this)
var chromeOptions = append(chromedp.DefaultExecAllocatorOptions[:],
	chromedp.Flag("headless", false), // todo debug
	chromedp.UserAgent(userAgent),
	chromedp.Flag("disable-popup-blocking", true),
)
var xcomUserCache *XcomUserInfo
var xcomDatabase interface {
	MetaGetValue(key string) string
	MetaUpdateValue(key, value string) error
}

func XcomTest() {
	email := host.GetEnvVar("X_EMAIL")
	username := host.GetEnvVar("X_USERNAME")
	password := host.GetEnvVar("X_PASSWORD")
	cookies, __error := LogInToXcom(email, username, password)
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
func LogInToXcom(email, username, password string) ([]*network.Cookie, error) {
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
	core.LogDebug("Navigating to X.com")
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
		return nil, core.LogDebugReturn("Username Entry Error: " + err.Error())
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
			return nil, core.LogDebugReturn("Email Entry Error: " + err2.Error())
		}
	}

	// Enter password
	err = chromedp.Run(ctx,
		chromedp.WaitVisible("input[type=\"password\"]", chromedp.ByQuery),
		slowType("input[type=\"password\"]", password+kb.Enter),
		chromedp.Sleep(time.Duration(security.RandomUint(15, 30))*time.Second),
	)
	if err != nil {
		return nil, core.LogDebugReturn("Password Entry Error: " + err.Error())
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
func CheckXcomCookiesAlive() {
	// todo check if the auth cookies are still alive
}
func RefreshXcomCookies() {
	// perform a request to /home to refresh the cookie expiration
}

func XcomTestCredentials(apiKey, apiSecret, accessToken, accessTokenSecret string) (bool, int) {
	if apiKey == "" || apiSecret == "" || accessToken == "" || accessTokenSecret == "" {
		return false, 0
	}
	oauth1Config := oauth1.NewConfig(apiKey, apiSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	httpClient := oauth1Config.Client(oauth1.NoContext, token)
	httpClient.Timeout = 10 * time.Second
	resp, err := httpClient.Get("https://api.twitter.com/2/users/me")
	if err != nil {
		core.LogDebug("X.com API test failed: " + err.Error())
		return false, 0
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		core.LogDebug("X.com API test succeeded")
		return true, resp.StatusCode
	}
	core.LogDebug("X.com API test failed with status: " + resp.Status)
	return false, resp.StatusCode
}
func XcomIsFreeTier(apiKey, apiSecret, accessToken, accessTokenSecret string) bool {
	if apiKey == "" || apiSecret == "" || accessToken == "" || accessTokenSecret == "" {
		return true
	}
	if xcomDatabase != nil {
		cachedTier := xcomDatabase.MetaGetValue("xcomIsFreeTier")
		if cachedTier == "true" {
			return true
		} else if cachedTier == "false" {
			return false
		}
	}
	userInfo, err := xcomGetCachedUser(apiKey, apiSecret, accessToken, accessTokenSecret)
	if err != nil {
		return true
	}
	oauth1Config := oauth1.NewConfig(apiKey, apiSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	httpClient := oauth1Config.Client(oauth1.NoContext, token)
	httpClient.Timeout = 10 * time.Second
	url := fmt.Sprintf("https://api.twitter.com/2/users/%s/reverse_chronological_timeline?max_results=1", userInfo.ID)
	resp, err := httpClient.Get(url)
	if err != nil {
		core.LogDebug("X.com tier check failed: " + err.Error())
		return true
	}
	defer resp.Body.Close()
	isFreeTier := true
	if resp.StatusCode == 200 {
		core.LogDebug("X.com API is paid tier (timeline access granted)")
		isFreeTier = false
	} else if resp.StatusCode == 403 {
		core.LogDebug("X.com API is free tier (timeline access denied)")
		isFreeTier = true
	} else {
		core.LogDebug("X.com tier check returned status: " + resp.Status)
	}
	if xcomDatabase != nil {
		if isFreeTier {
			xcomDatabase.MetaUpdateValue("xcomIsFreeTier", "true")
		} else {
			xcomDatabase.MetaUpdateValue("xcomIsFreeTier", "false")
		}
	}
	return isFreeTier
}
func XcomClearTierCache() {
	if xcomDatabase != nil {
		xcomDatabase.MetaUpdateValue("xcomIsFreeTier", "")
	}
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
func XcomSetDatabase(db interface {
	MetaGetValue(key string) string
	MetaUpdateValue(key, value string) error
}) {
	xcomDatabase = db
}
func XcomClearUserCache() {
	xcomUserCache = nil
	if xcomDatabase != nil {
		xcomDatabase.MetaUpdateValue("xcomUserID", "")
		xcomDatabase.MetaUpdateValue("xcomUsername", "")
		xcomDatabase.MetaUpdateValue("xcomName", "")
		xcomDatabase.MetaUpdateValue("xcomIsFreeTier", "")
	}
}
func xcomGetCachedUser(apiKey, apiSecret, accessToken, accessTokenSecret string) (*XcomUserInfo, error) {
	if xcomUserCache != nil {
		return xcomUserCache, nil
	}
	if xcomDatabase != nil {
		userID := xcomDatabase.MetaGetValue("xcomUserID")
		username := xcomDatabase.MetaGetValue("xcomUsername")
		name := xcomDatabase.MetaGetValue("xcomName")
		if userID != "" && username != "" {
			xcomUserCache = &XcomUserInfo{ID: userID, Username: username, Name: name}
			return xcomUserCache, nil
		}
	}
	oauth1Config := oauth1.NewConfig(apiKey, apiSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	httpClient := oauth1Config.Client(oauth1.NoContext, token)
	httpClient.Timeout = 30 * time.Second
	meResp, err := httpClient.Get("https://api.twitter.com/2/users/me?user.fields=username,name")
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get user, status: %s", meResp.Status)
	}
	var meData struct {
		Data struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(meResp.Body).Decode(&meData); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}
	xcomUserCache = &XcomUserInfo{
		ID:       meData.Data.ID,
		Username: meData.Data.Username,
		Name:     meData.Data.Name,
	}
	if xcomDatabase != nil {
		xcomDatabase.MetaUpdateValue("xcomUserID", xcomUserCache.ID)
		xcomDatabase.MetaUpdateValue("xcomUsername", xcomUserCache.Username)
		xcomDatabase.MetaUpdateValue("xcomName", xcomUserCache.Name)
	}
	return xcomUserCache, nil
}
func XcomGetHomeTimeline(apiKey, apiSecret, accessToken, accessTokenSecret string, maxResults int) ([]XcomPost, error) {
	if apiKey == "" || apiSecret == "" || accessToken == "" || accessTokenSecret == "" {
		return nil, fmt.Errorf("missing credentials")
	}
	userInfo, err := xcomGetCachedUser(apiKey, apiSecret, accessToken, accessTokenSecret)
	if err != nil {
		return nil, err
	}
	oauth1Config := oauth1.NewConfig(apiKey, apiSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	httpClient := oauth1Config.Client(oauth1.NoContext, token)
	httpClient.Timeout = 30 * time.Second
	url := fmt.Sprintf("https://api.twitter.com/2/users/%s/reverse_chronological_timeline?max_results=%d&tweet.fields=created_at,author_id&expansions=author_id&user.fields=username,name", userInfo.ID, maxResults)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get timeline, status: %s", resp.Status)
	}
	var timelineData struct {
		Data []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			AuthorID  string `json:"author_id"`
			CreatedAt string `json:"created_at"`
		} `json:"data"`
		Includes struct {
			Users []struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Name     string `json:"name"`
			} `json:"users"`
		} `json:"includes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&timelineData); err != nil {
		return nil, fmt.Errorf("failed to decode timeline response: %w", err)
	}
	userMap := make(map[string]struct{ Username, Name string })
	for _, user := range timelineData.Includes.Users {
		userMap[user.ID] = struct{ Username, Name string }{user.Username, user.Name}
	}
	var posts []XcomPost
	for _, tweet := range timelineData.Data {
		user := userMap[tweet.AuthorID]
		posts = append(posts, XcomPost{
			ID:        tweet.ID,
			Text:      tweet.Text,
			AuthorID:  tweet.AuthorID,
			CreatedAt: tweet.CreatedAt,
			Username:  user.Username,
			Name:      user.Name,
		})
	}
	return posts, nil
}
func XcomGetHomeTimelineScrape(email, username, password string, maxResults int) ([]XcomPost, error) {
	if email == "" || username == "" || password == "" {
		return nil, core.LogDebugReturn("missing scraping credentials")
	}
	core.LogDebug("Scraping X.com timeline")
	cookies, err := LogInToXcom(email, username, password)
	if err != nil {
		return nil, core.LogDebugReturn("failed to log in to X.com: " + err.Error())
	}
	if len(cookies) == 0 {
		return nil, core.LogDebugReturn("no cookies returned from login")
	}
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), chromeOptions...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, timeoutCancel := context.WithTimeout(ctx, 120*time.Second)
	defer timeoutCancel()
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			for _, cookie := range cookies {
				err := network.SetCookie(cookie.Name, cookie.Value).
					WithDomain(cookie.Domain).
					WithPath(cookie.Path).
					WithSecure(cookie.Secure).
					WithHTTPOnly(cookie.HTTPOnly).
					Do(ctx)
				if err != nil {
					return err
				}
			}
			return nil
		}),
		chromedp.Sleep(addTimeVariation(time.Duration(security.RandomUint(1, 3))*time.Second)),
		chromedp.Navigate("https://x.com/home"),
		chromedp.WaitVisible("article[data-testid=\"tweet\"]", chromedp.ByQuery),
		chromedp.Sleep(addTimeVariation(time.Duration(security.RandomUint(2, 4))*time.Second)),
		chromedp.EvaluateAsDevTools(`window.scrollBy(0, 500)`, nil),
		chromedp.Sleep(addTimeVariation(time.Duration(security.RandomUint(1, 3))*time.Second)),
		chromedp.EvaluateAsDevTools(`window.scrollBy(0, 300)`, nil),
		chromedp.Sleep(addTimeVariation(time.Duration(security.RandomUint(2, 4))*time.Second)),
	)
	if err != nil {
		return nil, core.LogDebugReturn("timeline navigation error: " + err.Error())
	}
	var posts []XcomPost
	var tweetsJSON string
	extractScript := fmt.Sprintf(`
		(function() {
			const tweets = document.querySelectorAll('article[data-testid="tweet"]');
			const results = [];
			const maxTweets = %d;
			for (let i = 0; i < Math.min(tweets.length, maxTweets); i++) {
				const tweet = tweets[i];
				const usernameEl = tweet.querySelector('a[href^="/"][role="link"] span');
				const textEl = tweet.querySelector('div[data-testid="tweetText"]');
				const timeEl = tweet.querySelector('time');
				if (textEl) {
					results.push({
						username: usernameEl ? usernameEl.textContent : '',
						text: textEl.textContent,
						createdAt: timeEl ? timeEl.getAttribute('datetime') : ''
					});
				}
			}
			return JSON.stringify(results);
		})()
	`, maxResults)
	err = chromedp.Run(ctx, chromedp.EvaluateAsDevTools(extractScript, &tweetsJSON))
	if err != nil {
		return nil, core.LogDebugReturn("tweet extraction error: " + err.Error())
	}
	var scrapedTweets []struct {
		Username  string `json:"username"`
		Text      string `json:"text"`
		CreatedAt string `json:"createdAt"`
	}
	if err := json.Unmarshal([]byte(tweetsJSON), &scrapedTweets); err != nil {
		return nil, core.LogDebugReturn("failed to parse scraped tweets: " + err.Error())
	}
	for i, tweet := range scrapedTweets {
		posts = append(posts, XcomPost{
			ID:        fmt.Sprintf("scraped-%d", i),
			Text:      tweet.Text,
			Username:  strings.TrimPrefix(tweet.Username, "@"),
			CreatedAt: tweet.CreatedAt,
		})
	}
	core.LogDebug(fmt.Sprintf("Scraped %d tweets from X.com", len(posts)))
	return posts, nil
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
