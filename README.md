# Sistem Distribuit - Telegram Bot (Media Downloader & Web Scraper)

## Ce face proiectul

Acest proiect reprezinta un sistem distribuit, bazat pe microservicii, care ofera o interfata de utilizare prin intermediul unui Bot de Telegram. Sistemul permite utilizatorilor sa execute doua functionalitati majore:
1. Descarcarea de continut media de pe YouTube, cu posibilitatea salvarii in format MP4 sau conversiei audio in MP3.
2. Rularea unui proces de Web Scraping asincron pe un URL specificat, extragand si deduplicand link-urile dintr-o pagina web.

Proiectul mentine de asemenea un istoric al actiunilor utilizatorilor in baza de date si implementeaza un mecanism avansat de rotatie a proxy-urilor pentru a asigura rezilienta crawler-ului in fata blocajelor de retea.

## Arhitectura si Module

Proiectul este structurat modular, fiecare componenta ruland in propriul container Docker pentru o decuplare totala:

* **Bot-ul (API Gateway):** Modulul de frontend care comunica cu API-ul Telegram prin webhook-uri. Gestioneaza rutarea comenzilor, paginarea meniurilor si executa operatiunile costisitoare de scriere pe disc a fisierelor media descarcate.
* **Dispatcher-ul (Orchestrator):** Actioneaza ca un strat middleware intre Bot si Crawler. Acesta preia cererile, extrage cel mai bun proxy disponibil din cache si delegeaza sarcina de procesare. Ulterior, stocheaza rezultatele parsate in format JSONB.
* **Crawler-ul (Worker):** Serviciul responsabil cu executia request-urilor HTTP de scraping. Foloseste tehnici de mascare (User-Agent spoofing, tunelare SOCKS5) si aplica expresii regulate pentru a parsa eficient codul sursa HTML.
* **Baza de Date:** Un container PostgreSQL folosit pentru stocarea istoricului de actiuni, a proxy-urilor disponibile si a link-urilor extrase de pe paginile web.
* **VPN Proxy:** Un serviciu SOCKS5 intern, utilizat pentru a masca adresa IP a aplicatiei in timpul procesului de web scraping.

## Design Pattern-uri Folosite

* **Strategy Pattern:**
  - **In modulul Bot:** A fost utilizat pentru a decupla rutarea mesajelor de executia lor. Interfata `ProcesatorStrategie` permite rularea actiunilor specifice fara a folosi o retea masiva de instructiuni if/else. Odata ce decizia a fost luata, sistemul doar apeleaza metoda polimorfica `Executa()`.
  - **In modulul Crawler:** Prin intermediul interfetei `IProxy`, codul poate executa HTTP fetch-uri folosind o conexiune directa sau mascata, fara ca logica business de web scraping sa fie constienta de implementarea de retea de dedesubt.
* **Simple Factory:**
  - In Bot, `ComandaBuilder` joaca rolul unei fabrici care analizeaza textul introdus de utilizator si returneaza instanta corecta de executie.
  - In Crawler, metoda `CreazaProxyConcret` evalueaza setul de date primit prin retea si instantiaza la runtime tipul corect de client de retea (`SocksProxy` sau `NoProxy`).
* **Singleton Pattern:**
  - Utilizat in implementarea structurii `ProxyManager` din Dispatcher. Apeland la primitiva `sync.Once`, sistemul garanteaza ca lista de proxy-uri este interogata si incarcata din baza de date in memoria RAM o singura data la pornirea serviciului, fiind ulterior accesibila global si sigur.

## Concepte Tehnice Cheie

* **Managementul Memoriei (Stream I/O):** Pentru a preveni consumul excesiv de memorie RAM la descarcarea videoclipurilor foarte mari, datele sunt transferate direct de la interfata de retea catre memoria non-volatila folosind pachetul standard `io.Copy`. Astfel, transferul se face prin buffere de dimensiuni mici, mentinand un consum de memorie constant.
* **Concurenta si Thread Safety:** Proiectul se foloseste de Goroutines pentru a procesa mesajele venite de la mai multi utilizatori simultan. Pentru a preveni fenomenele de Race Condition la accesarea resurselor globale partajate, starea a fost protejata cu `sync.Mutex` (pentru listele de proxy) si `sync.RWMutex` (pentru cache-ul de link-uri paginate).
* **Inter-Process Communication (OS Child Processes):** Conversia fisierelor video (.mp4) in fisiere audio (.mp3) se face delegand operatiunea catre un proces extern din sistemul de operare gazda (FFmpeg), instantiat prin pachetul `os/exec`.
* **Stateless UI:** Paginarea in meniul Telegram se face fara mentinerea starii pe server. Informatia de context (ID-ul cautarii si indexul paginii) este serializata si incapsulata in string-ul butoanelor de tip Callback (`p|ID|Pagina`), asigurand un consum zero de memorie intre interogarile utilizatorilor.

## Cum se porneste proiectul

Proiectul este containerizat complet folosind Docker, facand instalarea si pornirea extrem de simple.

### 1. Cerinte preliminare
Asigurati-va ca aveti instalate pe sistemul gazda urmatoarele utilitare:
* Docker
* Docker Compose

### 2. Configurarea mediului
In directorul radacina al proiectului, este necesar sa setati variabilele de mediu pentru bot. Puteti exporta aceste variabile in terminal sau sa creati configuratia necesara de mediu:

```bash
export TELEGRAM_TOKEN="token-ul-primit-de-la-botfather"
export WEBHOOK_URL="url-ul-serverului-expus-public"
```

### 3. Pornirea sistemului
Deschideti terminalul in locatia unde se afla fisierul `docker-compose.yml` si rulati comanda de build si start. Aceasta comanda va porni baza de date, va rula scriptul de initializare a tabelelor si va porni toate microserviciile in retea, in regim de fundal:

```bash
docker compose up --build -d
```

### 4. Oprirea sistemului
Pentru a opri si a inlatura containerele si retelele create, rulati comanda:

```bash
docker compose down
```
