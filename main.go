package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var sounds = map[string][][]byte{}

func main() {

	// load all the sounds
	log.Print("loading sounds")
	configData, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}
	config := map[string]string{}
	err = json.Unmarshal(configData, &config)
	if err != nil {
		log.Fatal(err)
	}
	for key, value := range config {
		sounds[key] = loadSound(value)
	}
	// connect to discord
	log.Print("joining discord")
	discord, err := discordgo.New("Bot " + os.Getenv("TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	defer discord.Close()

	log.Print("adding handler")
	discord.Identify.Intents = discordgo.IntentsGuildVoiceStates
	discord.AddHandler(serverJoin)

	log.Print("creating websocket")
	err = discord.Open()
	if err != nil {
		log.Fatal(err)
	}
	log.Print("waiting for close")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func loadSound(path string) [][]byte {
	buffer := [][]byte{}
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}

	var opuslen int16
	for {
		err = binary.Read(file, binary.LittleEndian, &opuslen)
		// If this is the end of the file, just return.
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			err := file.Close()
			if err != nil {
				log.Fatal(err)
			}
			break
		}

		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			log.Fatal(err)
		}

		// Read encoded pcm from dca file.
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)

		// Should not be any end of file errors
		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			log.Fatal(err)
		}

		// Append encoded pcm data to the buffer.
		buffer = append(buffer, InBuf)
	}

	return buffer
}

func serverJoin(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	log.Print("event received")
	if m.UserID == s.State.User.ID {
		return
	}
	if !(m.BeforeUpdate == nil && m.ChannelID != "") {
		return
	}
	buffer, ok := sounds[m.UserID]
	if !ok {
		log.Print("could not find user", m.UserID)
		return
	}
	voice, err := s.ChannelVoiceJoin(m.GuildID, m.ChannelID, false, false)
	if err != nil {
		log.Print(err.Error())
		return
	}
	err = voice.Speaking(true)
	if err != nil {
		log.Print(err.Error())
		return
	}
	for _, buff := range buffer {
		voice.OpusSend <- buff
	}

	err = voice.Speaking(false)
	if err != nil {
		log.Print(err.Error())
		return
	}
	voice.Disconnect()
}
