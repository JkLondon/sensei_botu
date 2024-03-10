package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var sensei = os.Getenv("SENSEI_NUMBER")

type bot struct {
	wa *whatsmeow.Client
	tg *tgbotapi.BotAPI
}

func (b *bot) eventHandler(evt interface{}) {
	sensei = os.Getenv("SENSEI_NUMBER")
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.Sender.User == sensei {
			messageType := v.Info.Type
			idStr := os.Getenv("GROUP_ID")
			id, _ := strconv.ParseInt(idStr, 10, 64)
			println("id", id, idStr)
			if v.Message.GetExtendedTextMessage() != nil {
				messageType = "url"
			}
			switch messageType {
			case "media":
				fmt.Println("Received a media message from the sensei!", v.Info.Sender.User, v.Info.Type)
				message, mType := ToDownLoadableMessage(v.Message)
				data, err := b.wa.Download(message)
				if err != nil {
					println("Error downloading file:", err)
				}
				println("Downloaded file:", data)
				switch mType {
				case "image":
					photo := tgbotapi.NewPhoto(id, tgbotapi.FileBytes{
						Name:  v.Message.GetImageMessage().GetCaption(),
						Bytes: data,
					})
					photo.Caption = v.Message.GetImageMessage().GetCaption()
					b.tg.Send(photo)
				case "document":
					document := tgbotapi.NewDocument(id, tgbotapi.FileBytes{
						Name:  v.Message.GetDocumentMessage().GetFileName(),
						Bytes: data,
					})
					document.Caption = v.Message.GetDocumentMessage().GetCaption()
					b.tg.Send(document)
				case "video":
					video := tgbotapi.NewVideo(id, tgbotapi.FileBytes{
						Name:  v.Message.GetVideoMessage().GetCaption(),
						Bytes: data,
					})
					video.Caption = v.Message.GetVideoMessage().GetCaption()
					b.tg.Send(video)
				default:
					document := tgbotapi.NewDocument(id, tgbotapi.FileBytes{
						Name:  "somewhat",
						Bytes: data,
					})
					document.Caption = "somewhat"
					b.tg.Send(document)
				}

			case "text":
				fmt.Println("Received a text message from the sensei!", v.Info.Sender.User, v.Info.Type, v.Message.GetConversation())
				b.tg.Send(tgbotapi.NewMessage(id, v.Message.GetConversation()))
			case "url":
				b.tg.Send(tgbotapi.NewMessage(id, v.Message.GetExtendedTextMessage().GetText()))
			}
		}
		fmt.Println("Received a message!", v.Message.GetConversation(), v.Info.Sender.User, v.Info.Type, sensei)

	}
}

func main() {
	godotenv.Load(".env")
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	// Make sure you add appropriate DB connector imports, e.g. github.com/mattn/go-sqlite3 for SQLite
	container, err := sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	tgBot, err := tgbotapi.NewBotAPI(os.Getenv("TG_BOT_TOKEN"))
	if err != nil {
		panic(err)
	}

	tgBot.Debug = true
	b := &bot{wa: client, tg: tgBot}
	client.AddEventHandler(b.eventHandler)

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code here
				// e.g. qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				// or just manually `echo 2@... | qrencode -t ansiutf8` in a terminal
				fmt.Println("QR code:", evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}

func ToDownLoadableMessage(message *proto.Message) (whatsmeow.DownloadableMessage, string) {
	if message.GetImageMessage() != nil {
		return message.GetImageMessage(), "image"
	}
	if message.GetDocumentMessage() != nil {
		return message.GetDocumentMessage(), "document"
	}
	if message.GetVideoMessage() != nil {
		return message.GetVideoMessage(), "video"
	}
	if message.GetAudioMessage() != nil {
		return message.GetAudioMessage(), "audio"
	}
	return nil, "nil"
}
