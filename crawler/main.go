package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"comun"

	_ "github.com/lib/pq"
)

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/proceseaza", handleProceseaza)
	http.ListenAndServe("0.0.0.0:8081", nil)
}

type IProxy interface {
	ConstruiesteClient() (*http.Client, error)
}

type SocksProxy struct {
	Tip    string
	Adresa string
	Nume   string
	Parola string
}

func (s *SocksProxy) ConstruiesteClient() (*http.Client, error) {
	var proxyString string
	if s.Nume != "" || s.Parola != "" {
		proxyString = fmt.Sprintf("%s://%s:%s@%s", s.Tip, s.Nume, s.Parola, s.Adresa)
	} else {
		proxyString = fmt.Sprintf("%s://%s", s.Tip, s.Adresa)
	}

	proxyURL, err := url.Parse(proxyString)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   15 * time.Second,
	}, nil
}

type NoProxy struct{}

func (n *NoProxy) ConstruiesteClient() (*http.Client, error) {
	return &http.Client{
		Timeout: 15 * time.Second,
	}, nil
}

func CreazaProxyConcret(date comun.ProxyData) (IProxy, error) {
	if date.Adresa == "" {
		return &NoProxy{}, nil
	}
	switch date.Tip {
	case "socks4", "socks5":
		return &SocksProxy{Tip: date.Tip, Adresa: date.Adresa, Nume: date.Nume, Parola: date.Parola}, nil
	default:
		return nil, fmt.Errorf("tip de proxy nesuportat momentan: %s", date.Tip)
	}
}

func handleProceseaza(w http.ResponseWriter, r *http.Request) {
	var cerere comun.CerereCrawler
	json.NewDecoder(r.Body).Decode(&cerere)

	w.Header().Set("Content-Type", "application/json")

	proxyConcret, err := CreazaProxyConcret(cerere.Proxy)
	if err != nil {
		json.NewEncoder(w).Encode(comun.RaspunsCrawler{Mesaj: "Eroare instantiere proxy: " + err.Error()})
		return
	}

	client, err := proxyConcret.ConstruiesteClient()
	if err != nil {
		json.NewEncoder(w).Encode(comun.RaspunsCrawler{Mesaj: "Eroare construire client HTTP: " + err.Error()})
		return
	}

	htmlBrut, err := ExtrageHTML(client, cerere.URL)
	if err != nil {
		json.NewEncoder(w).Encode(comun.RaspunsCrawler{Mesaj: "Eroare Scraping: " + err.Error()})
		return
	}

	listaLinkuri := ExtrageLinkuri(htmlBrut, cerere.URL)

	limita := min(len(listaLinkuri), 10)

	for i := 0; i < limita; i++ {
		db.Exec(`INSERT INTO rezultate_extragere (link_sursa, link_gasit) VALUES ($1, $2)`, cerere.URL, listaLinkuri[i])
	}

	mesaj := fmt.Sprintf("S-au gasit %d link-uri", len(listaLinkuri))

	raspuns := comun.RaspunsCrawler{
		Mesaj:   mesaj,
		Linkuri: listaLinkuri,
	}

	json.NewEncoder(w).Encode(raspuns)
}

func ExtrageHTML(client *http.Client, link string) (string, error) {
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func ExtrageLinkuri(html string, linkSursa string) []string {
	re := regexp.MustCompile(`href="(http[s]?://[^"]+|/[^"]+)"`)
	matches := re.FindAllStringSubmatch(html, -1)

	parsedURL, _ := url.Parse(linkSursa)
	domeniu := parsedURL.Scheme + "://" + parsedURL.Host

	linkuriUnice := make(map[string]bool)
	var listaFinala []string

	for _, match := range matches {
		if len(match) > 1 {
			link := match[1]

			if strings.Contains(link, ".css") || strings.Contains(link, ".ico") ||
				strings.Contains(link, ".png") || strings.Contains(link, ".woff") ||
				strings.Contains(link, ".js") || strings.Contains(link, ".woff2") {
				continue
			}

			if len(link) > 0 && link[0] == '/' {
				if len(link) > 1 && link[1] == '/' {
					link = "https:" + link
				} else {
					link = domeniu + link
				}
			}

			if !linkuriUnice[link] {
				linkuriUnice[link] = true
				listaFinala = append(listaFinala, link)
			}
		}
	}

	return listaFinala
}
