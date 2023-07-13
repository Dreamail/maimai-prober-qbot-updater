package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
)

type Bot struct {
	*http.Client
	OnUserIDUpdate func(*Bot)
}

var (
	ua = "Mozilla/5.0 (Linux; U; UOS x86_64; zh-cn) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 UOSBrowser/6.0.1.1001"
)

func (bot *Bot) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", ua)
	oldUserID := GetCookieByName(bot.Jar.Cookies(req.URL), "userId").Value
	resp, err := bot.Client.Do(req)
	if err != nil {
		return resp, err
	}
	newUserID := GetCookieByName(bot.Jar.Cookies(req.URL), "userId").Value
	if oldUserID != newUserID {
		bot.OnUserIDUpdate(bot)
	}
	return resp, err
}

func NewBotClient(token *MaiToken, refresh func() *MaiToken, onUserIDUpdate func(*Bot)) (*Bot, error) {
	if token == nil {
		fmt.Println("no token, refresh it")
		token = refresh()
	}
	if token == nil {
		return nil, errors.New("get token err")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	url, _ := url.Parse("https://maimai.wahlap.com/")
	jar.SetCookies(
		url,
		[]*http.Cookie{
			{
				Name:   "_t",
				Value:  token.Token,
				Path:   "/",
				Domain: "maimai.wahlap.com",
			},
			{
				Name:   "userId",
				Value:  token.UserID,
				Path:   "/",
				Domain: "maimai.wahlap.com",
			},
		})
	bot := &Bot{
		Client: &http.Client{
			Jar: jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if via[0].Method != "GET" {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		OnUserIDUpdate: onUserIDUpdate,
	}
	if bot.IsTokenExpired() {
		maiToken := refresh()
		if maiToken == nil {
			return nil, errors.New("refresh token err")
		}
		bot.Jar.SetCookies(
			url,
			[]*http.Cookie{
				{
					Name:   "_t",
					Value:  maiToken.Token,
					Path:   "/",
					Domain: "maimai.wahlap.com",
				},
				{
					Name:   "userId",
					Value:  maiToken.UserID,
					Path:   "/",
					Domain: "maimai.wahlap.com",
				},
			})
	}

	_, err = gocron.NewScheduler(time.Local).Every("5m").Do(func() {
		if bot.IsTokenExpired() {
			maiToken := refresh()
			if maiToken == nil {
				return
			}
			bot.Jar.SetCookies(
				url,
				[]*http.Cookie{
					{
						Name:   "_t",
						Value:  maiToken.Token,
						Path:   "/",
						Domain: "maimai.wahlap.com",
					},
					{
						Name:   "userId",
						Value:  maiToken.UserID,
						Path:   "/",
						Domain: "maimai.wahlap.com",
					},
				})
		}
	})
	if err != nil {
		return nil, err
	}

	return bot, nil
}
func (bot *Bot) GetMaiToken() *MaiToken {
	url, _ := url.Parse("https://maimai.wahlap.com/")
	cookies := bot.Jar.Cookies(url)
	token := GetCookieByName(cookies, "_t")
	userID := GetCookieByName(cookies, "userId")
	return &MaiToken{
		Token:  token.Value,
		UserID: userID.Value,
	}
}

func (bot *Bot) IsTokenExpired() bool {
	req, err := http.NewRequest("HAED", "https://maimai.wahlap.com/maimai-mobile/home/", nil)
	if err != nil {
		return true
	}
	resp, err := bot.Do(req)
	if err != nil {
		return true
	}
	resp.Body.Close()
	return resp.StatusCode != 200
}
func ValidateBody(body string) error {
	if strings.Contains(body, "错误码：") {
		code := regexp.MustCompile(`错误码：\d*`).FindString(body)
		return errors.New(code)
	}
	return nil
}

func (bot *Bot) sendFriendApi(uri, idx string, extra url.Values) (*http.Response, error) {
	form := url.Values{}
	url, _ := url.Parse("https://maimai.wahlap.com/")
	cookies := bot.Jar.Cookies(url)
	tokenCookie := GetCookieByName(cookies, "_t")
	if tokenCookie == nil {
		return nil, errors.New("token not found")
	}
	form.Add("token", tokenCookie.Value)
	form.Add("idx", idx)
	for k, v := range extra {
		form.Add(k, v[0])
	}

	req, err := http.NewRequest("POST", "https://maimai.wahlap.com"+uri, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := bot.Do(req)
	if err != nil {
		return nil, err
	}

	resp.Body.Close()
	if resp.StatusCode != 200 {
		if resp.StatusCode != 302 {
			return nil, errors.New("api request failed: " + resp.Status)
		}
		// I dont know if 302 is a error, orz
		/*if !strings.Contains(resp.Header.Get("Location"), "index.php") {
			return nil, errors.New("api request failed: 302 found, but got err")
		}*/
	}
	return resp, nil
}

func (bot *Bot) ValidateFriendCode(idx string) error {
	req, err := http.NewRequest("GET", "https://maimai.wahlap.com/maimai-mobile/friend/search/searchUser/?friendCode="+idx, nil)
	if err != nil {
		return err
	}
	resp, err := bot.Do(req)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	err = ValidateBody(string(body))
	if err != nil {
		return errors.New("validate friend code failed: " + err.Error())
	}
	if strings.Contains(string(body), "找不到该玩家") {
		return errors.New("player was not found")
	}
	return nil
}

func (bot *Bot) SendFriendRequest(idx string) error {
	_, err := bot.sendFriendApi("/maimai-mobile/friend/search/invite/", idx, url.Values{
		"invite": []string{""},
	})
	return err
}

func (bot *Bot) CancelFriendRequest(idx string) error {
	_, err := bot.sendFriendApi("/maimai-mobile/friend/invite/cancel/", idx, url.Values{
		"invite": []string{""},
	})
	return err
}

func (bot *Bot) GetSentFriendRequest() ([]string, error) {
	req, err := http.NewRequest("GET", "https://maimai.wahlap.com/maimai-mobile/friend/invite/", nil)
	if err != nil {
		return nil, err
	}
	resp, err := bot.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	err = ValidateBody(string(body))
	if err != nil {
		return nil, errors.New("get sent request list failed: " + err.Error())
	}
	ts := regexp.MustCompile(`<input type="hidden" name="idx" value="(.*?)"`).FindAllStringSubmatch(string(body), -1)
	var ids []string
	for _, v := range ts {
		ids = append(ids, v[1])
	}
	return ids, nil
}

// May not work :)
func (bot *Bot) AllowFriendRequest(idx string) error {
	_, err := bot.sendFriendApi("/maimai-mobile/friend/friend/accept/allow/", idx, url.Values{
		"allow": []string{""},
	})
	return err
}

// May not work :)
func (bot *Bot) BlockFriendRequest(idx string) error {
	_, err := bot.sendFriendApi("/maimai-mobile/friend/friend/accept/block/", idx, url.Values{
		"block": []string{""},
	})
	return err
}

func (bot *Bot) RemoveFriend(idx string) error {
	_, err := bot.sendFriendApi("/maimai-mobile/friend/friendDetail/drop/", idx, nil)
	return err
}

func (bot *Bot) GetFriendList() ([]string, error) {
	req, err := http.NewRequest("GET", "https://maimai.wahlap.com/maimai-mobile/friend/", nil)
	if err != nil {
		return nil, err
	}
	resp, err := bot.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	err = ValidateBody(string(body))
	if err != nil {
		return nil, errors.New("get friend list failed: " + err.Error())
	}
	ts := regexp.MustCompile(`<input type="hidden" name="idx" value="(.*?)"`).FindAllStringSubmatch(string(body), -1)
	var ids []string
	for _, v := range ts {
		ids = append(ids, v[1])
	}
	return ids, nil
}

func (bot *Bot) FavoriteOnFriend(idx string) error {
	_, err := bot.sendFriendApi("/maimai-mobile/friend/favoriteOn/", idx, nil)
	return err
}

func (bot *Bot) FavoriteOffFriend(idx string) error {
	_, err := bot.sendFriendApi("/maimai-mobile/friend/favoriteOff/", idx, nil)
	return err
}

func (bot *Bot) GetFriendVS(idx, scoreType string, diff int, onlyLose bool) (string, error) {
	form := url.Values{}
	form.Add("genre", "99")
	form.Add("scoreType", scoreType)
	form.Add("diff", fmt.Sprint(diff))
	form.Add("idx", idx)
	if onlyLose {
		form.Add("loseOnly", "on")
	}

	req, err := http.NewRequest("GET", "https://maimai.wahlap.com/maimai-mobile/friend/friendGenreVs/battleStart/?"+form.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := bot.Do(req)
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", err
	}
	err = ValidateBody(string(body))
	if err != nil {
		return "", errors.New("get friend vs failed: " + err.Error())
	}
	return string(body), nil
}
