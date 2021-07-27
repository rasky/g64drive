package main

import (
	_ "embed"
	"strings"

	"gopkg.in/ini.v1"
)

//go:embed "data/mupen64plus.ini"
var romdb []byte

type RomDBGame struct {
	Name     string
	CRC      string
	Mempak   bool
	Rumble   bool
	SaveType string
}

func romdb_search(rommd5 string) RomDBGame {
	cfg, err := ini.Load(romdb)
	if err != nil {
		panic(err)
	}

	game := RomDBGame{}

	sec := cfg.Section(strings.ToUpper(rommd5))
	game.Name = sec.Key("GoodName").String()
	game.CRC = sec.Key("CRC").String()

	refmd5 := sec.Key("RefMD5").String()
	if refmd5 != "" {
		sec = cfg.Section(refmd5)
	}

	game.Mempak, _ = sec.Key("Mempak").Bool()
	game.Rumble, _ = sec.Key("Rumble").Bool()
	game.SaveType = sec.Key("SaveType").String()

	return game
}
