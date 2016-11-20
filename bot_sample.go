// Copyright (c) 2016 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/mattermost/platform/model"
	"gopkg.in/yaml.v2"
)

const (
	SAMPLE_NAME = "Mattermost Bot Sample"

	USER_NAME  = "samplebot"
	USER_FIRST = "Sample"
	USER_LAST  = "Bot"

	TEAM_NAME        = "test"
	CHANNEL_LOG_NAME = "debugging-for-sample-bot"
)

var userEmail, userPassword string

var client *model.Client
var webSocketClient *model.WebSocketClient

var botUser *model.User
var botTeam *model.Team
var initialLoad *model.InitialLoad
var debuggingChannel *model.Channel

// Settings is a struct for load settings
type Settings struct {
	UserEmail    string `yaml:"user_email"`
	UserPassword string `yaml:"user_password"`
	TeamName     string `yaml:"team_name"`
}

var s Settings

// Documentation for the Go driver can be found
// at https://godoc.org/github.com/mattermost/platform/model#Client
func main() {
	println(SAMPLE_NAME)
	buf, err := ioutil.ReadFile("settings.yml")
	if err != nil {
		panic(err)
	}

	s = Settings{}
	//m := make(map[interface{}]interface{})
	err = yaml.Unmarshal(buf, &s)
	if err != nil {
		panic(err)
	}
	userEmail = s.UserEmail
	fmt.Printf("settings: %v", s)
	//fmt.Printf("%s\n", s["user_email"])

	SetupGracefulShutdown()

	client = model.NewClient("http://localhost:8065")

	// Lets test to see if the mattermost server is up and running
	MakeSureServerIsRunning()

	// lets attempt to login to the Mattermost server as the bot user
	// This will set the token required for all future calls
	// You can get this token with client.AuthToken
	LoginAsTheBotUser()

	// If the bot user doesn't have the correct information lets update his profile
	//UpdateTheBotUserIfNeeded()

	// Lets load all the stuff we might need
	InitialLoad()

	// Lets find our bot team
	FindBotTeam()

	// This is an important step.  Lets make sure we use the botTeam
	// for all future web service requests that require a team.
	client.SetTeamId(botTeam.Id)

	GetChannels()

	// Lets create a bot channel for logging debug messages into
	//CreateBotDebuggingChannelIfNeeded()
	//SendMsgToDebuggingChannel("_"+SAMPLE_NAME+" has **started** running_", "")

	// Lets start listening to some channels via the websocket!
	/*
		webSocketClient, err := model.NewWebSocketClient("ws://localhost:8065", client.AuthToken)
		if err != nil {
			println("We failed to connect to the web socket")
			PrintError(err)
		}

		webSocketClient.Listen()

		go func() {
			for {
				select {
				case resp := <-webSocketClient.EventChannel:
					HandleWebSocketResponse(resp)
				}
			}
		}()

		// You can block forever with

		select {}
	*/
}

func MakeSureServerIsRunning() {
	if props, err := client.GetPing(); err != nil {
		println("There was a problem pinging the Mattermost server.  Are you sure it's running?")
		PrintError(err)
		os.Exit(1)
	} else {
		println("Server detected and is running version " + props["version"])
	}
}

func LoginAsTheBotUser() {
	if loginResult, err := client.Login(s.UserEmail, s.UserPassword); err != nil {
		println("There was a problem logging into the Mattermost server.  Are you sure ran the setup steps from the README.md?")
		PrintError(err)
		os.Exit(1)
	} else {
		botUser = loginResult.Data.(*model.User)
	}
}

func UpdateTheBotUserIfNeeded() {
	if botUser.FirstName != USER_FIRST || botUser.LastName != USER_LAST || botUser.Username != USER_NAME {
		botUser.FirstName = USER_FIRST
		botUser.LastName = USER_LAST
		botUser.Username = USER_NAME

		if updateUserResult, err := client.UpdateUser(botUser); err != nil {
			println("We failed to update the Sample Bot user")
			PrintError(err)
			os.Exit(1)
		} else {
			botUser = updateUserResult.Data.(*model.User)
			println("Looks like this might be the first run so we've updated the bots account settings")
		}
	}
}

func InitialLoad() {
	if initialLoadResults, err := client.GetInitialLoad(); err != nil {
		println("We failed to get the initial load")
		PrintError(err)
		os.Exit(1)
	} else {
		initialLoad = initialLoadResults.Data.(*model.InitialLoad)
	}
}

func FindBotTeam() {
	for _, team := range initialLoad.Teams {
		if team.Name == s.TeamName {
			botTeam = team
			break
		}
	}

	if botTeam == nil {
		println("We do not appear to be a member of the team '" + s.TeamName + "'")
		os.Exit(1)
	}
}

func GetChannels() {
	if channelsResult, err := client.GetChannels(""); err != nil {
		PrintError(err)
	} else {
		channelList := channelsResult.Data.(*model.ChannelList)
		for _, channel := range *channelList {
			println(channel.DisplayName)
			GetMessages(channel.Id)
		}
	}
}

func GetMessages(channelId string) {
	if postsResult, err := client.GetPostsSince(channelId, 0); err != nil {
		PrintError(err)
	} else {
		postList := postsResult.Data.(*model.PostList)
		for _, post := range postList.Posts {
			println(post.Message)
			fmt.Println(time.Unix(0, post.CreateAt*int64(time.Millisecond)))
			println("%f", post.CreateAt)
			fmt.Printf("%f", post.CreateAt)

		}
	}

}

func CreateBotDebuggingChannelIfNeeded() {
	if channelsResult, err := client.GetChannels(""); err != nil {
		println("We failed to get the channels")
		PrintError(err)
	} else {
		channelList := channelsResult.Data.(*model.ChannelList)
		for _, channel := range *channelList {

			// The logging channel has alredy been created, lets just use it
			if channel.Name == CHANNEL_LOG_NAME {
				debuggingChannel = channel
				return
			}
		}
	}

	// Looks like we need to create the logging channel
	channel := &model.Channel{}
	channel.Name = CHANNEL_LOG_NAME
	channel.DisplayName = "Debugging For Sample Bot"
	channel.Purpose = "This is used as a test channel for logging bot debug messages"
	channel.Type = model.CHANNEL_OPEN
	if channelResult, err := client.CreateChannel(channel); err != nil {
		println("We failed to create the channel " + CHANNEL_LOG_NAME)
		PrintError(err)
	} else {
		debuggingChannel = channelResult.Data.(*model.Channel)
		println("Looks like this might be the first run so we've created the channel " + CHANNEL_LOG_NAME)
	}
}

func SendMsgToDebuggingChannel(msg string, replyToId string) {
	post := &model.Post{}
	post.ChannelId = debuggingChannel.Id
	post.Message = msg

	post.RootId = replyToId

	if _, err := client.CreatePost(post); err != nil {
		println("We failed to send a message to the logging channel")
		PrintError(err)
	}
}

func HandleWebSocketResponse(event *model.WebSocketEvent) {
	HandleMsgFromDebuggingChannel(event)
}

func HandleMsgFromDebuggingChannel(event *model.WebSocketEvent) {
	// If this isn't the debugging channel then lets ingore it
	if event.Broadcast.ChannelId != debuggingChannel.Id {
		return
	}

	// Lets only reponded to messaged posted events
	if event.Event != model.WEBSOCKET_EVENT_POSTED {
		return
	}

	println("responding to debugging channel msg")

	post := model.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post != nil {

		// ignore my events
		if post.UserId == botUser.Id {
			return
		}

		// if you see any word matching 'alive' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)alive(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}

		// if you see any word matching 'up' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)up(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}

		// if you see any word matching 'running' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)running(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}

		// if you see any word matching 'hello' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)hello(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}
	}

	SendMsgToDebuggingChannel("I did not understand you!", post.Id)
}

func PrintError(err *model.AppError) {
	println("\tError Details:")
	println("\t\t" + err.Message)
	println("\t\t" + err.Id)
	println("\t\t" + err.DetailedError)
}

func SetupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			if webSocketClient != nil {
				webSocketClient.Close()
			}

			SendMsgToDebuggingChannel("_"+SAMPLE_NAME+" has **stopped** running_", "")
			os.Exit(0)
		}
	}()
}
