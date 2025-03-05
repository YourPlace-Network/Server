package services

import (
	"YourPlace/src/core"
	"context"
	"fmt"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"time"
)

type Tweet struct {
	Text   string `json:"text"`
	Author string `json:"author"`
}
type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type Account struct {
	ID    string `json:"id"`
	Apps  []App  `json:"apps"`
	Email string `json:"email"`
}
type FeedItem struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	UserID    string `json:"userId"`
}
type AttachAppRequest struct {
	AccountID string `json:"accountId"`
	AppID     string `json:"appId"`
}

// In-memory storage // TODO: Replace with database
var accounts = make(map[string]Account)
var apps = make(map[string]App)
var feedItems = make(map[string]FeedItem)

func AttachTwitterAppHandler(c *gin.Context) {
	var req AttachAppRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.SecureJSON(http.StatusBadRequest, gin.H{"status": "Invalid request body"})
		return
	}
	account, exists := accounts[req.AccountID]
	if !exists {
		c.SecureJSON(http.StatusNotFound, gin.H{"status": "Account not found"})
		return
	}
	app, exists := apps[req.AppID]
	if !exists {
		c.SecureJSON(http.StatusNotFound, gin.H{"status": "App not found"})
		return
	}
	// Check if app is already attached
	for _, a := range account.Apps {
		if a.ID == app.ID {
			c.SecureJSON(http.StatusOK, gin.H{"status": "success"})
			return
		}
	}
	// Attach app to account
	account.Apps = append(account.Apps, app)
	accounts[req.AccountID] = account
	c.SecureJSON(http.StatusOK, gin.H{"status": "success", "account": account})
}
func ProfileFeedHandler(c *gin.Context) {
	accountID := c.Query("accountId")
	if accountID == "" {
		c.SecureJSON(http.StatusBadRequest, gin.H{"status": "Account ID is required"})
		return
	}
	_, exists := accounts[accountID]
	if !exists {
		c.SecureJSON(http.StatusNotFound, gin.H{"status": "Account not found"})
		return
	}
	feed, exists := feedItems[accountID]
	if !exists {
		c.SecureJSON(http.StatusOK, gin.H{"status": "success", "feed": []FeedItem{}})
		return
	}
	c.SecureJSON(http.StatusOK, gin.H{"status": "success", "feed": feed})
}

func GetTweets(username string, password string) {
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.3"),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.WindowSize(1920, 1080),
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), options...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second) // create timeout for entire execution
	defer cancel()

	var cookies []*network.Cookie
	err := chromedp.Run(ctx,
		chromedp.Navigate("https://x.com//login"), // Navigate to login page
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, _ = network.GetCookies().Do(ctx)
			core.LogInfo("Cookies: " + fmt.Sprintf("%+v", cookies))
			return nil
		}),
		chromedp.WaitVisible("//input[@autocomplete='username']"),        // Wait for username field
		chromedp.SendKeys("//input[@autocomplete='username']", username), // Type username
		chromedp.ActionFunc(func(ctx context.Context) error {
			return debugStep(ctx, "1_initial_load")
		}),
		//chromedp.Click("//div[@role='button']//span[text()='Next']"),     // Click next button
		//chromedp.WaitVisible(`//input[@name='password']`),                // Wait for password field
		//chromedp.SendKeys(`//input[@name='password']`, password),         // Type password
		//chromedp.Click(`//div[@role='button']//span[text()='Log in']`),   // Click login button
		//chromedp.WaitVisible(`//div[@data-testid='primaryColumn']`),      // Wait for home page to load
	)
	if err != nil {
		core.LogError("Twitter Browser Error 1: " + err.Error())
		return
	}

	return // debug

	var tweets []Tweet
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`div[data-testid="tweet"]`),
		chromedp.Evaluate(`
            Array.from(document.querySelectorAll('div[data-testid="tweet"]')).map(tweet => {
                return {
                    text: tweet.querySelector('div[data-testid="tweetText"]')?.innerText,
                    author: tweet.querySelector('div[data-testid="User-Name"]')?.innerText
                }
            })
        `, &tweets),
	)
	if err != nil {
		core.LogError("Twitter Browser Error 2: " + err.Error())
		return
	}
	for _, tweet := range tweets {
		core.LogInfo("Tweet: " + tweet.Text + " by " + tweet.Author)
	}
}

func debugStep(ctx context.Context, stepName string) error {
	// Get page HTML
	var html string
	if err := chromedp.Run(ctx, chromedp.InnerHTML("html", &html)); err != nil {
		return fmt.Errorf("getting HTML for step %s: %w", stepName, err)
	}
	// Save HTML to file
	filename := fmt.Sprintf("debug_%s.html", stepName)
	if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
		return fmt.Errorf("saving HTML for step %s: %w", stepName, err)
	}
	// Take screenshot
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 90)); err != nil {
		return fmt.Errorf("taking screenshot for step %s: %w", stepName, err)
	}
	// Save screenshot
	if err := os.WriteFile(fmt.Sprintf("screenshot_%s.png", stepName), buf, 0644); err != nil {
		return fmt.Errorf("saving screenshot for step %s: %w", stepName, err)
	}
	log.Printf("Completed step: %s - saved HTML and screenshot\n", stepName)
	return nil
}
