# maimai-prober-qbot-updater
[README_EN](https://github.com/Dreamail/maimai-prober-qbot-updater/blob/main/README.md)

通过QQ机器人更新maimai成绩，基于maimai友人对战实现

README还在写，可能不够完善，如遇到问题欢迎提交issue或来个pr

# Deploy
## Requirements
* Go-CQHTTP
* 一个微信账号（用于登陆maimaiBot）
* A Brain（XD
## Step
0. Go-CQHTTP配置不在此阐述，详见其文档
1. 去Actions下载你所需平台的预构建二进制，或自行编译项目。
2. 首次在终端运行二进制，会自动在当前目录生成`config.json`配置文件
3. 修改配置文件：

| Name | Description |
| ---- | ----------- |
| tokenFile | maimai登陆token文件储存路径 |
| userFile | 用户绑定储存路径 |
| groupID | 限制可以使用的群组，为0则不限制 |
| zero | ZeroBot配置，详见其文档 |
| ws | Go-CQHTTP的WebSocket地址 |
| accessToken | Go-CQHTTP AccessToken |

Note: 如果ZereBot CommandPrefix后不带空格，如`/`，命令将会为`/bindmai`等；如带空格，如默认配置`/mai `，命令将会为`/mai bindmai`等

4. 再次运行二进制，首次登陆或maimai token失效会向管理员账号发送微信登陆二维码，请登陆maimai bot微信账号，或提前抓包手动修改tokenFile
5. 不出意外就部署完成啦

## Q&A
TODO

# User Usage
## Commands

| Command    | Description    |
|------------|----------------|
| bindmaimai | 绑定maimai账号 |
| bindprober | 绑定查分器账号 |
| update     | 更新maimai成绩 |

Note: 
1. 每个命令都需要添加命令前缀，前缀在`config.json`中指定
2. 在允许群组或向Bot发送命令前缀可获取详细用法

# Thanks To
* [Diving-Fish/maimaidx-prober](https://github.com/Diving-Fish/maimaidx-prober)
* [bakapiano/maimaidx-prober-proxy-updater](https://github.com/bakapiano/maimaidx-prober-proxy-updater)
* [eatmoreapple/openwechat](https://github.com/eatmoreapple/openwechat)
* [wdvxdr1123/ZeroBot](https://github.com/wdvxdr1123/ZeroBot)