package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/driver"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var (
	config *BoConfig
	bot    *Bot
)

type BoConfig struct {
	TokenFile   string      `json:"tokenFile"`
	UserFile    string      `json:"userFile"`
	GroupID     int64       `json:"groupID"`
	Zero        zero.Config `json:"zero"`
	Ws          string      `json:"ws"`
	AccessToken string      `json:"accessToken"`
}

func LoadConfig(path string) (*BoConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, errors.New("config file is not found")
	}
	configFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()
	config := &BoConfig{}
	err = json.NewDecoder(configFile).Decode(&config)
	config.Zero.Driver = []zero.Driver{driver.NewWebSocketClient(config.Ws, config.AccessToken)}
	return config, err
}
func NewConfigFile(path string) error {
	configFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer configFile.Close()
	config := &BoConfig{
		Zero: zero.Config{
			NickName:   []string{},
			SuperUsers: []int64{},
		},
	}
	return json.NewEncoder(configFile).Encode(config)
}

func main() {
	configFile := "config.json"
	var err error
	config, err = LoadConfig(configFile)
	if err != nil {
		if err.Error() != "config file is not found" {
			panic(err)
		}
		err = NewConfigFile(configFile)
		if err != nil {
			panic(err)
		}
		fmt.Println("no config file found, created!")
		os.Exit(0)
	}

	maiToken, err := LoadToken(config.TokenFile)
	if err != nil && err.Error() != "token file is not found" {
		panic(err)
	}

	err = StartQBot()
	if err != nil {
		panic(err)
	}

	bot, err = NewBotClient(
		maiToken,
		func() *MaiToken {
			maiToken, err := RefreashToken(func(uuid string) {
				SendToSuper(
					message.Image("https://login.weixin.qq.com/qrcode/"+uuid),
					message.Text("maibot: token expired"),
				)
			})
			if err != nil {
				SendToSuper(message.Text("maibot: refresh token failed: " + err.Error()))
				return nil
			}
			SendToSuper(message.Text("maibot: refresh token success"))
			maiToken.SaveToken(config.TokenFile)
			return maiToken
		},
		func(bot *Bot) {
			bot.GetMaiToken().SaveToken(config.TokenFile)
		},
	)
	if err != nil {
		panic(err)
	}

	select {}
}
