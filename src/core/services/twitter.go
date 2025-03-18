package services

import (
	"YourPlace/src/core"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"context"
	"fmt"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"math/rand"
	"os"
	"strings"
	"time"
)

type Post struct {
	Username string
	Content  string
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.3"

var typingSpeed = addTimeVariation(200 * time.Millisecond)
var chromeOptions = append(chromedp.DefaultExecAllocatorOptions[:],
	chromedp.Flag("headless", false), // todo debug
	chromedp.UserAgent(userAgent),
	chromedp.Flag("disable-popup-blocking", true),
)

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

func slowType(selector, text string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		for _, char := range text {
			err := chromedp.SendKeys(selector, string(char), chromedp.ByQuery).Do(ctx)
			if err != nil {
				return err
			}
			time.Sleep(typingSpeed)
		}
		return nil
	})
}
func addTimeVariation(d time.Duration) time.Duration {
	variation := (rand.Float64() - 0.5) * 0.4
	return d + time.Duration(float64(d)*variation)
}
