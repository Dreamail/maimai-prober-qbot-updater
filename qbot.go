package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/shell"
	"github.com/wdvxdr1123/ZeroBot/message"
	"golang.org/x/exp/slices"
)

type MaiUser struct {
	FriendID, Username, Passwd string
	Updating                   bool `json:"-"`
}

var (
	bindUsage = "bindmai 好友代码 [f]\n" +
		" - 绑定你的maimai账号，f选项用于重新绑定"
	bindProberUsage = "bindprober 查分器账号 查分器密码 [f]\n" +
		" - 绑定你的查分器账号，f选项用于重新绑定"
	updateUsage = "update [查分器账号] [查分器密码]\n" +
		" - 如已绑定查分器账户，无需提供账号密码"

	userMap = make(map[int64]MaiUser)
)

func SaveUserMap(userMap map[int64]MaiUser, path string) error {
	userFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer userFile.Close()
	return json.NewEncoder(userFile).Encode(userMap)
}
func LoadUserMap(path string) (map[int64]MaiUser, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[int64]MaiUser), errors.New("user map file is not found")
	}
	userFile, err := os.Open(path)
	if err != nil {
		return make(map[int64]MaiUser), err
	}
	defer userFile.Close()
	var userMap map[int64]MaiUser
	err = json.NewDecoder(userFile).Decode(&userMap)
	return userMap, err
}

func RestrictRule(ctx *zero.Ctx) bool {
	if ctx.Event.GroupID == 0 {
		result := ctx.GetGroupMemberInfo(config.GroupID, ctx.Event.UserID, false)
		return result.String() != ""
	}
	return true
}

func StartQBot() error {
	zero.Run(&config.Zero)

	prefix := config.Zero.CommandPrefix
	if prefix == "" {
		return errors.New("qbot: prefix must be set")
	}
	bindUsage = prefix + bindUsage
	bindProberUsage = prefix + bindProberUsage
	updateUsage = prefix + updateUsage

	zero.OnCommand("bindmai", RestrictRule).Handle(onBindMai)
	zero.OnCommand("bindprober", RestrictRule).Handle(onBindProber)
	zero.OnCommand("update", RestrictRule).Handle(onUpdateRecords)
	zero.OnFullMatch(strings.TrimSpace(prefix), RestrictRule).Handle(onMai)

	var err error
	userMap, err = LoadUserMap(config.UserFile)
	if err != nil {
		if err.Error() != "user map file is not found" {
			return err
		}
	}
	return nil
}

func SendToSuper(msg ...interface{}) {
	zero.RangeBot(func(id int64, ctx *zero.Ctx) bool {
		for _, v := range zero.BotConfig.SuperUsers {
			for _, m := range msg {
				ctx.SendPrivateMessage(v, m)
			}
		}
		return true
	})
}

func onBindMai(ctx *zero.Ctx) {
	args := shell.Parse(ctx.State["args"].(string))
	if len(args) != 1 && len(args) != 2 {
		ctx.Send(message.Text(
			"参数错误，用法：（方括号内为可选参数）\n",
			bindUsage,
		))
		return
	}
	maiUser, ok := userMap[ctx.Event.UserID]
	if ok && userMap[ctx.Event.UserID].FriendID != "" && (len(args) != 2 || args[1] != "f") {
		ctx.Send(message.Text("你已经绑定了啦，需要重新绑定请添加f选项"))
		return
	}

	err := bot.ValidateFriendCode(args[0])
	if err != nil {
		if err.Error() == "player was not found" {
			ctx.Send(message.Text(zero.BotConfig.NickName[0] + "找不到你哟"))
		} else {
			SendToSuper(message.Text("on bind: " + err.Error()))
			ctx.Send(message.Text(zero.BotConfig.NickName[0] + "找你找出错啦，请稍后再试或联系管理员"))
		}
		return
	}

	var friendList, sentList []string // for goto
	friendList, err = bot.GetFriendList()
	if err != nil {
		ctx.Send(message.Text("添加好友失败了，请稍后再试或联系管理员"))
		SendToSuper(message.Text("on bind: " + err.Error()))
		return
	}
	if slices.Contains(friendList, args[0]) {
		ctx.Send(message.Text("你已经是好友了，所以绑定成功啦！"))
		goto ret
	}

	sentList, err = bot.GetSentFriendRequest()
	if err != nil {
		ctx.Send(message.Text("添加好友失败了，请稍后再试或联系管理员"))
		SendToSuper(message.Text("on bind: " + err.Error()))
		return
	}
	if slices.Contains(sentList, args[0]) {
		ctx.Send(message.Text("已经给你发过好友请求了啦，同意好友申请就完成绑定啦！"))
		goto ret
	}

	err = bot.SendFriendRequest(args[0])
	if err != nil {
		ctx.Send(message.Text("发送好友请求失败，请稍后再试或联系管理员"))
		SendToSuper(message.Text("on bind: " + err.Error()))
		return
	}
	ctx.Send(message.Text(zero.BotConfig.NickName[0] + "给你发送好友请求啦，同意好友申请就完成绑定啦！"))

ret:
	maiUser.FriendID = args[0]

	userMap[ctx.Event.UserID] = maiUser

	err = SaveUserMap(userMap, config.UserFile)
	if err != nil {
		SendToSuper(message.Text("on bind: " + err.Error()))
		return
	}
}
func onBindProber(ctx *zero.Ctx) {
	args := shell.Parse(ctx.State["args"].(string))
	if len(args) != 2 && len(args) != 3 {
		ctx.Send(message.Text(
			"参数错误，用法：（方括号内为可选参数）\n",
			bindProberUsage,
		))
		return
	}
	maiUser, ok := userMap[ctx.Event.UserID]
	if ok && userMap[ctx.Event.UserID].Username != "" && (len(args) != 3 || args[2] != "f") {
		ctx.Send(message.Text("你已经绑定了啦，需要重新绑定请添加f选项"))
		return
	}

	loginResp, err := http.Post(
		"https://www.diving-fish.com/api/maimaidxprober/login",
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"username": "%s", "password": "%s"}`, args[0], args[1])),
	)
	if err != nil {
		ctx.Send(message.Text("登陆查分器出错啦，请稍后再试或联系管理员"))
		SendToSuper(message.Text("on bind: " + err.Error()))
		return
	}
	if loginResp.StatusCode == 401 {
		ctx.Send(message.Text("查分器账号密码有误，请检查一下哦"))
		return
	}
	if loginResp.StatusCode != 200 {
		ctx.Send(message.Text("登陆查分器出错啦，请稍后再试或联系管理员"))
		body, err := io.ReadAll(loginResp.Body)
		loginResp.Body.Close()
		if err != nil {
			SendToSuper(message.Text("on bind: " + err.Error()))
			return
		}
		SendToSuper(message.Text("on bind: " + string(strings.TrimSpace(string(body)))))
		return
	}
	ctx.Send(message.Text("绑定查分器账号成功！"))

	maiUser.Username = args[0]
	maiUser.Passwd = args[1]

	userMap[ctx.Event.UserID] = maiUser

	err = SaveUserMap(userMap, config.UserFile)
	if err != nil {
		SendToSuper(message.Text("on bind: " + err.Error()))
		return
	}
}
func onUpdateRecords(ctx *zero.Ctx) {
	args := shell.Parse(ctx.State["args"].(string))
	if len(args) != 0 && len(args) != 2 {
		ctx.Send(message.Text(
			"参数错误，用法：（方括号内为可选参数）\n",
			updateUsage,
		))
		return
	}

	maiUser := userMap[ctx.Event.UserID]
	if maiUser.FriendID == "" {
		ctx.Send(message.Text("你还未绑定maimai账户，先进行一个账户绑定吧！"))
		return
	}

	username := ""
	passwd := ""
	if len(args) == 2 {
		username = args[0]
		passwd = args[1]
	} else {
		username = maiUser.Username
		passwd = maiUser.Passwd
	}
	if username == "" {
		ctx.Send(message.Text("你还未绑定查分器账户，请提供查分器帐密或者先进行一个账户绑定吧！"))
		return
	}

	err := bot.FavoriteOnFriend(maiUser.FriendID)
	if err != nil {
		ctx.Send(message.Text("把你登陆到喜爱失败惹，请稍后再试或联系管理员"))
		SendToSuper(message.Text("on update: " + err.Error()))
		return
	}

	status, _ := bot.UpdateScore(maiUser.FriendID, username, passwd, true)

	ctx.Send(message.Text("开始更新成绩～"))
	errStr := ""
	for i := 0; i < 5; i++ {
		stat := <-status
		if strings.Contains(stat, "err: ") {
			errStr = stat
			break
		}
		if ctx.Event.GroupID == 0 {
			ctx.Send(message.Text(stat))
		}
	}
	if errStr != "" {
		ctx.Send(message.Text("更新时出现问题，请稍后再试或联系管理员"))
		SendToSuper(message.Text("on update: " + errStr))
		return
	}
	ctx.Send(message.Text("所有成绩更新完成！"))

	err = bot.FavoriteOffFriend(maiUser.FriendID)
	if err != nil {
		SendToSuper(message.Text("on update: " + err.Error()))
		return
	}
}
func onMai(ctx *zero.Ctx) {
	ctx.Send(message.Text(
		"用法：（方括号内为可选参数）",
		"\n",
		bindUsage,
		"\n",
		bindProberUsage,
		"\n",
		updateUsage,
	))
}
