package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"

	"github.com/eatmoreapple/openwechat"
)

type MaiToken struct {
	Token, UserID string
}

func (token MaiToken) SaveToken(path string) error {
	tokenFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer tokenFile.Close()
	return json.NewEncoder(tokenFile).Encode(token)
}
func LoadToken(path string) (*MaiToken, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, errors.New("token file is not found")
	}
	tokenFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer tokenFile.Close()
	maiToken := &MaiToken{}
	err = json.NewDecoder(tokenFile).Decode(&maiToken)
	return maiToken, err
}

func RefreashToken(uuidCallBack func(uuid string)) (*MaiToken, error) {
	bot := openwechat.DefaultBot(openwechat.Desktop)
	defer bot.Logout()

	bot.UUIDCallback = uuidCallBack

	err := bot.Login()
	if err != nil {
		return nil, err
	}

	authURL, err := getRedirectLocation(bot.Caller.Client.Do, "https://tgk-wcaime.wahlap.com/wc_auth/oauth/authorize/maimai-dx")
	if err != nil {
		return nil, err
	}
	callBackURL, err := getRedirectLocation(bot.Caller.Client.Do, "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxcheckurl?requrl="+url.QueryEscape(authURL))
	if err != nil {
		return nil, err
	}
	maiURL, err := getRedirectLocation(bot.Caller.Client.Do, callBackURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", maiURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bot.Caller.Client.Do(req)
	if err != nil {
		return nil, err
	}

	cg := openwechat.CookieGroup(resp.Cookies())
	token, exist := cg.GetByName("_t")
	if !exist {
		return nil, err
	}
	userID, exist := cg.GetByName("userId")
	if !exist {
		return nil, err
	}

	return &MaiToken{
		Token:  token.Value,
		UserID: userID.Value,
	}, nil
}

func getRedirectLocation(do func(req *http.Request) (*http.Response, error), url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 301 && resp.StatusCode != 302 {
		return "", errors.New("is not a redirect respone")
	}
	return resp.Header.Get("Location"), nil
}
