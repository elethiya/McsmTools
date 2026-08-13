package players

import (
	"encoding/json"
	"os"
	"path/filepath"

	"void-panel/pkg/config"
	"void-panel/pkg/mcserver"
)

type PlayerInfo struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Level       int    `json:"level,omitempty"`       // for ops
	Bypasses    bool   `json:"bypassesPlayerLimit,omitempty"`
	Reason      string `json:"reason,omitempty"`      // for bans
	Created     string `json:"created,omitempty"`     // for bans
	Source      string `json:"source,omitempty"`      // for bans
	Expires     string `json:"expires,omitempty"`     // for bans
}

type PlayerListData struct {
	Ops     []PlayerInfo `json:"ops"`
	White   []PlayerInfo `json:"whitelist"`
	Banned  []PlayerInfo `json:"banned"`
}

func GetPlayerLists() (*PlayerListData, error) {
	cfg := config.GlobalConfig
	serverDir := "./mc_server"
	if cfg != nil {
		serverDir = cfg.ServerDir
	}

	data := &PlayerListData{
		Ops:    readJSONList(filepath.Join(serverDir, "ops.json")),
		White:  readJSONList(filepath.Join(serverDir, "whitelist.json")),
		Banned: readJSONList(filepath.Join(serverDir, "banned-players.json")),
	}

	return data, nil
}

func readJSONList(filePath string) []PlayerInfo {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []PlayerInfo{}
	}

	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return []PlayerInfo{}
	}

	var list []PlayerInfo
	if err := json.Unmarshal(bytes, &list); err != nil {
		return []PlayerInfo{}
	}

	return list
}

func OpPlayer(name string) error {
	return mcserver.GetManager().SendCommand("op " + name)
}

func DeopPlayer(name string) error {
	return mcserver.GetManager().SendCommand("deop " + name)
}

func WhitelistAdd(name string) error {
	return mcserver.GetManager().SendCommand("whitelist add " + name)
}

func WhitelistRemove(name string) error {
	return mcserver.GetManager().SendCommand("whitelist remove " + name)
}

func KickPlayer(name, reason string) error {
	cmd := "kick " + name
	if reason != "" {
		cmd += " " + reason
	}
	return mcserver.GetManager().SendCommand(cmd)
}

func BanPlayer(name, reason string) error {
	cmd := "ban " + name
	if reason != "" {
		cmd += " " + reason
	}
	return mcserver.GetManager().SendCommand(cmd)
}

func UnbanPlayer(name string) error {
	return mcserver.GetManager().SendCommand("pardon " + name)
}
