package main

import (
	"bytes"
	"comun"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kkdai/youtube/v2"
	_ "github.com/lib/pq"
)

var (
	cacheLinkuri = make(map[string][]string)
	mutexCache   sync.RWMutex
	dbBot        *sql.DB
)

const LinkuriPePagina = 10
const IstoricPePagina = 5

type ProcesatorStrategie interface {
	Executa(bot *tgbotapi.BotAPI, message *tgbotapi.Message)
}

type StrategieYouTube struct {
	URL string
}

func (s *StrategieYouTube) Executa(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	proceseazaYouTube(bot, message)
}

type StrategieScraping struct {
	URL string
}

func (s *StrategieScraping) Executa(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	salveazaInIstoric(message.Chat.ID, message.Text, "Scraper")
	proceseazaScraping(bot, message)
}

type StrategieIstoric struct{}

func (s *StrategieIstoric) Executa(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	trimitePaginaIstoric(bot, message.Chat.ID, 0, 0)
}

type ComandaBuilder struct {
	text string
}

func NewComandaBuilder(text string) *ComandaBuilder {
	return &ComandaBuilder{text: text}
}

func (b *ComandaBuilder) Construieste() ProcesatorStrategie {
	if b.text == "/istoric" {
		return &StrategieIstoric{}
	}
	if strings.Contains(b.text, "youtube.com") || strings.Contains(b.text, "youtu.be") {
		return &StrategieYouTube{URL: b.text}
	}
	return &StrategieScraping{URL: b.text}
}

func main() {
	var err error
	dbBot, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Println("Avertisment: Nu s-a putut conecta la baza de date din bot:", err)
	}

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	webhookConfig, err := tgbotapi.NewWebhook(os.Getenv("WEBHOOK_URL"))
	if err != nil {
		log.Fatal(err)
	}

	_, err = bot.Request(webhookConfig)
	if err != nil {
		log.Fatal(err)
	}

	updates := bot.ListenForWebhook("/")

	go http.ListenAndServe("0.0.0.0:8080", nil)

	for update := range updates {
		if update.Message != nil {
			go proceseazaRutare(bot, update.Message)
		} else if update.CallbackQuery != nil {
			go proceseazaCallback(bot, update.CallbackQuery)
		}
	}
}

func salveazaInIstoric(chatID int64, comanda string, tip string) {
	if dbBot != nil {
		_, err := dbBot.Exec(`INSERT INTO istoric_utilizatori (chat_id, comanda, tip_comanda) VALUES ($1, $2, $3)`, chatID, comanda, tip)
		if err != nil {
			log.Println("Eroare la salvare istoric:", err)
		}
	}
}

func trimitePaginaIstoric(bot *tgbotapi.BotAPI, chatID int64, messageID int, pagina int) {
	if dbBot == nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Baza de date nu este disponibila"))
		return
	}

	limit := IstoricPePagina
	offset := pagina * limit

	rows, err := dbBot.Query(`SELECT comanda, tip_comanda FROM istoric_utilizatori WHERE chat_id = $1 ORDER BY id DESC LIMIT $2 OFFSET $3`, chatID, limit+1, offset)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Eroare la preluarea istoricului"))
		return
	}
	defer rows.Close()

	type Inregistrare struct {
		Cmd string
		Tip string
	}
	var inregistrari []Inregistrare

	for rows.Next() {
		var r Inregistrare
		if err := rows.Scan(&r.Cmd, &r.Tip); err == nil {
			inregistrari = append(inregistrari, r)
		}
	}

	if err := rows.Err(); err != nil {
		log.Println("Eroare in timpul iterarii istoricului:", err)
	}

	areNext := len(inregistrari) > limit
	if areNext {
		inregistrari = inregistrari[:limit]
	}

	if len(inregistrari) == 0 && pagina == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "Nu ai nicio actiune salvata in istoric"))
		return
	}

	textMesaj := fmt.Sprintf("<b>Istoric: (Pagina %d):</b>\n\n", pagina+1)
	for _, r := range inregistrari {
		textMesaj += fmt.Sprintf("%s\n%s\n\n", r.Tip, r.Cmd)
	}

	var randButoane []tgbotapi.InlineKeyboardButton

	if pagina > 0 {
		dataBack := fmt.Sprintf("h|%d", pagina-1)
		randButoane = append(randButoane, tgbotapi.NewInlineKeyboardButtonData("Inapoi", dataBack))
	}

	if areNext {
		dataNext := fmt.Sprintf("h|%d", pagina+1)
		randButoane = append(randButoane, tgbotapi.NewInlineKeyboardButtonData("Inainte", dataNext))
	}

	var msg tgbotapi.MessageConfig
	var editMsg tgbotapi.EditMessageTextConfig

	if messageID == 0 {
		msg = tgbotapi.NewMessage(chatID, textMesaj)
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		if len(randButoane) > 0 {
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(randButoane...))
		}
		bot.Send(msg)
	} else {
		editMsg = tgbotapi.NewEditMessageText(chatID, messageID, textMesaj)
		editMsg.ParseMode = "HTML"
		editMsg.DisableWebPagePreview = true
		if len(randButoane) > 0 {
			markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(randButoane...))
			editMsg.ReplyMarkup = &markup
		}
		bot.Send(editMsg)
	}
}

func proceseazaRutare(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	builder := NewComandaBuilder(message.Text)
	strategie := builder.Construieste()
	strategie.Executa(bot, message)
}

func proceseazaCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	bot.Request(tgbotapi.NewCallback(callback.ID, ""))

	if strings.HasPrefix(callback.Data, "p|") {
		proceseazaPaginare(bot, callback)
		return
	}

	if strings.HasPrefix(callback.Data, "h|") {
		date := strings.Split(callback.Data, "|")
		if len(date) == 2 {
			pagina, _ := strconv.Atoi(date[1])
			trimitePaginaIstoric(bot, callback.Message.Chat.ID, callback.Message.MessageID, pagina)
		}
		return
	}

	date := strings.Split(callback.Data, "|")
	if len(date) != 2 {
		return
	}

	tip := date[0]
	videoID := date[1]

	msgAsteptare := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, "Se descarca...")
	bot.Send(msgAsteptare)

	descarcaMedia(bot, callback.Message.Chat.ID, callback.Message.MessageID, videoID, tip)
}

func proceseazaScraping(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	cerere := comun.CerereInitala{
		ChatID: message.Chat.ID,
		URL:    message.Text,
	}

	body, _ := json.Marshal(cerere)
	resp, err := http.Post("http://dispatcher_aplicatie:8082/dispatch", "application/json", bytes.NewBuffer(body))
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Eroare: Dispatcher offline."))
		return
	}
	defer resp.Body.Close()

	var raspuns comun.RaspunsCrawler
	json.NewDecoder(resp.Body).Decode(&raspuns)

	if len(raspuns.Linkuri) == 0 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Nu s-au gasit link-uri."))
		return
	}

	cautareID := fmt.Sprintf("%d", message.MessageID)

	mutexCache.Lock()
	cacheLinkuri[cautareID] = raspuns.Linkuri
	mutexCache.Unlock()

	trimitePagina(bot, message.Chat.ID, 0, cautareID, 0)
}

func trimitePagina(bot *tgbotapi.BotAPI, chatID int64, messageID int, cautareID string, pagina int) {
	mutexCache.RLock()
	linkuri := cacheLinkuri[cautareID]
	mutexCache.RUnlock()

	totalLinkuri := len(linkuri)
	start := pagina * LinkuriPePagina

	end := start + LinkuriPePagina
	if end > totalLinkuri {
		end = totalLinkuri
	}

	textMesaj := fmt.Sprintf("Link-urile gasite (Pagina %d):\n", pagina+1)

	for i := start; i < end; i++ {
		textMesaj += fmt.Sprintf("- %s\n", linkuri[i])
	}

	var randButoane []tgbotapi.InlineKeyboardButton

	if pagina > 0 {
		dataBack := fmt.Sprintf("p|%s|%d", cautareID, pagina-1)
		randButoane = append(randButoane, tgbotapi.NewInlineKeyboardButtonData("Inapoi", dataBack))
	}

	if end < totalLinkuri {
		dataNext := fmt.Sprintf("p|%s|%d", cautareID, pagina+1)
		randButoane = append(randButoane, tgbotapi.NewInlineKeyboardButtonData("Inainte", dataNext))
	}

	var msg tgbotapi.MessageConfig
	var editMsg tgbotapi.EditMessageTextConfig

	if messageID == 0 {
		msg = tgbotapi.NewMessage(chatID, textMesaj)
		if len(randButoane) > 0 {
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(randButoane...))
		}
		bot.Send(msg)
	} else {
		editMsg = tgbotapi.NewEditMessageText(chatID, messageID, textMesaj)
		if len(randButoane) > 0 {
			markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(randButoane...))
			editMsg.ReplyMarkup = &markup
		}
		bot.Send(editMsg)
	}
}

func proceseazaPaginare(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	date := strings.Split(callback.Data, "|")
	if len(date) != 3 {
		return
	}

	cautareID := date[1]
	paginaStr := date[2]
	pagina, _ := strconv.Atoi(paginaStr)

	trimitePagina(bot, callback.Message.Chat.ID, callback.Message.MessageID, cautareID, pagina)
}

func proceseazaYouTube(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	client := youtube.Client{}
	video, err := client.GetVideo(message.Text)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Eroare la citirea video ului: "+err.Error()))
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Titlu video: "+video.Title+"\n\nAlege formatul:")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Video (MP4)", "v|"+video.ID),
			tgbotapi.NewInlineKeyboardButtonData("Audio (MP3)", "m|"+video.ID),
		),
	)
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
}

func descarcaMedia(bot *tgbotapi.BotAPI, chatID int64, msgID int, videoID string, tip string) {
	client := youtube.Client{}
	video, err := client.GetVideo(videoID)
	if err != nil {
		bot.Send(tgbotapi.NewEditMessageText(chatID, msgID, "Eroare la citirea video ului"))
		return
	}

	urlVideo := "https://www.youtube.com/watch?v=" + videoID
	numeFormat := "YouTube MP4"
	if tip == "m" {
		numeFormat = "YouTube MP3"
	}
	salveazaInIstoric(chatID, urlVideo, numeFormat)

	var format youtube.Format
	formats := video.Formats.WithAudioChannels()

	if tip == "m" {
		audioFormats := video.Formats.Type("audio")
		if len(audioFormats) > 0 {
			format = audioFormats[0]
		} else if len(formats) > 0 {
			format = formats[0]
		}
	} else {
		if len(formats) > 0 {
			format = formats[0]
		}
	}

	if format.URL == "" && format.ItagNo == 0 {
		bot.Send(tgbotapi.NewEditMessageText(chatID, msgID, "Nu s a gasit un format valid cu audio"))
		return
	}

	stream, _, err := client.GetStream(video, &format)
	if err != nil {
		bot.Send(tgbotapi.NewEditMessageText(chatID, msgID, "Eroare la preluarea stream ului"))
		return
	}
	defer stream.Close()

	extensie := ".mp4"
	if tip == "m" {
		extensie = ".m4a"
	}
	numeFisier := fmt.Sprintf("media_temp_%d%s", chatID, extensie)

	file, err := os.Create(numeFisier)
	if err != nil {
		bot.Send(tgbotapi.NewEditMessageText(chatID, msgID, "Eroare la salvarea temp ului"))
		return
	}

	_, err = io.Copy(file, stream)
	file.Close()

	defer os.Remove(numeFisier)

	if err != nil {
		bot.Send(tgbotapi.NewEditMessageText(chatID, msgID, "Eroare la salvarea video ului pe disc: "+err.Error()))
		return
	}

	fisierDeTrimis := numeFisier

	if tip == "m" {
		numeFisierMP3 := fmt.Sprintf("media_temp_%d.mp3", chatID)
		cmd := exec.Command("ffmpeg", "-i", numeFisier, "-q:a", "0", "-map", "a", numeFisierMP3, "-y")

		err = cmd.Run()
		if err != nil {
			bot.Send(tgbotapi.NewEditMessageText(chatID, msgID, "Eroare la conversia FFmpeg: "+err.Error()))
			return
		}

		fisierDeTrimis = numeFisierMP3
		defer os.Remove(numeFisierMP3)
	}

	info, err := os.Stat(fisierDeTrimis)
	if err == nil && info.Size() > 49*1024*1024 {
		bot.Send(tgbotapi.NewEditMessageText(chatID, msgID, "Video ul depaseste 50 mb si deci nu poate fi trimis pe Telegram"))
		return
	}

	bot.Send(tgbotapi.NewEditMessageText(chatID, msgID, "Se urca fisierul pe Telegram..."))

	if tip == "m" {
		msgAudio := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(fisierDeTrimis))
		msgAudio.Caption = "Titlu audio:" + video.Title
		_, err = bot.Send(msgAudio)
	} else {
		msgVideo := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(fisierDeTrimis))
		msgVideo.Caption = "Titlu video:" + video.Title
		_, err = bot.Send(msgVideo)
	}

	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Eroare la trimiterea video ului pe Telegram"))
	} else {
		bot.Request(tgbotapi.NewDeleteMessage(chatID, msgID))
	}
}
