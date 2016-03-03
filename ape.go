package ape

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/websocket"
)

type callbackFunc func(*Event)

type Command struct {
	name string
	args []string
}

func (c *Command) Name() string {
	return c.name
}

func (c *Command) Args() []string {
	return c.args
}

func newCommand(name string, args []string) *Command {
	return &Command{
		name: name,
		args: args,
	}
}

type Event struct {
	data    map[string]interface{}
	command *Command
	Nick    string
}

func (e *Event) Command() *Command {
	return e.command
}

func (e *Event) Message() string {
	if message, ok := e.data["text"]; ok && message != nil {
		return message.(string)
	}
	return ""
}

func (e *Event) targetName() string {
	pattern := `^([^:]+): `
	matches := regexp.MustCompile(pattern).FindStringSubmatch(e.Message())
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (e *Event) messageWithoutName() string {
	pattern := fmt.Sprintf(`^(%s: )`, e.targetName())
	message := regexp.MustCompile(pattern).ReplaceAllString(e.Message(), "")
	return strings.TrimSpace(message)
}

func (e *Event) buildCommand() {
	args := strings.Split(e.messageWithoutName(), " ")
	e.command = newCommand(args[0], args[1:])
}

func newEvent(data map[string]interface{}) *Event {
	return &Event{
		data: data,
	}
}

type Connection struct {
	token         string
	userId        string
	userName      string
	channel       string
	initActions   []callbackFunc
	defaultAction callbackFunc
	actions       map[string]callbackFunc
	userMap       map[string]string
}

func (con *Connection) Channel() string {
	return con.channel
}

func (con *Connection) RegisterChannel(channel string) {
	con.channel = channel
}

func (con *Connection) SendMessage(message string) {
	resp, err := http.PostForm("https://slack.com/api/chat.postMessage", url.Values{
		"token":   {con.token},
		"channel": {con.channel},
		"text":    {message},
		"as_user": {"true"},
	})
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
}

func (con *Connection) AddCallback(eventCode string, callback callbackFunc) string {
	return ""
}

func (con *Connection) AddInitAction(callback callbackFunc) {
	con.initActions = append(con.initActions, callback)
}

func (con *Connection) AddDefaultAction(callback callbackFunc) {
	con.defaultAction = callback
}

func (con *Connection) AddAction(command string, callback callbackFunc) {
	con.actions[command] = callback
}

func NewConnection(token string) *Connection {
	return &Connection{
		token:         token,
		channel:       "",
		initActions:   []callbackFunc{},
		defaultAction: nil,
		actions:       map[string]callbackFunc{},
	}
}

func (con *Connection) Loop() {
	ws, err := con.newWSConnection()
	if err != nil {
		panic(err)
	}

	for {
		var data map[string]interface{}
		websocket.JSON.Receive(ws, &data)
		switch data["type"] {
		case "hello":
			e := newEvent(data)
			e.buildCommand()
			for _, callback := range con.initActions {
				callback(e)
			}
		case "message":
			if subtype, ok := data["subtype"]; ok && subtype == "bot_message" {
				break
			}
			e := newEvent(data)
			if userId, ok := data["user"]; ok && userId != nil {
				if username, ok := con.userMap[userId.(string)]; ok {
					e.Nick = username
				}
			}
			name := e.targetName()
			if name != con.userName && name != "<@"+con.userId+">" {
				break
			}
			e.buildCommand()
			if callback, ok := con.actions[e.Command().Name()]; ok {
				callback(e)
			}
			if con.defaultAction != nil {
				con.defaultAction(e)
			}
		}
	}
}

func (con *Connection) newWSConnection() (*websocket.Conn, error) {
	resp, err := http.PostForm("https://slack.com/api/rtm.start", url.Values{"token": {con.token}})
	if err != nil {
		return nil, err
	}

	var r struct {
		Ok    bool   `json:"ok"`
		Url   string `json:"url"`
		Error string `json:"error"`
		Self  struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"self"`
		Users []struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"users"`
	}

	err = json.NewDecoder(resp.Body).Decode(&r)
	if err != nil {
		return nil, err
	}

	con.userId = r.Self.Id
	con.userName = r.Self.Name
	userMap := map[string]string{}
	for _, user := range r.Users {
		userMap[user.Id] = user.Name
	}
	con.userMap = userMap

	return websocket.Dial(r.Url, "", "https://slack.com/")
}
