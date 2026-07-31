# Distributed System - Telegram Bot (Media Downloader & Web Scraper)

## Project Overview

This project represents a distributed, microservices-based system that provides a user interface through a Telegram Bot. The system allows users to execute two major functionalities:
1. Downloading media content from YouTube, with options for saving in MP4 format or converting the audio to MP3.
2. Running an asynchronous Web Scraping process on a specified URL, extracting and deduplicating links from a web page.

The project also maintains a log of user actions in a database and implements an advanced proxy rotation mechanism to ensure crawler resilience against network blocks.

## Architecture and Modules

The project is modularly structured, with each component running in its own Docker container for complete decoupling:

* **Bot (API Gateway):** The frontend module that communicates with the Telegram API via webhooks. It manages command routing, menu pagination, and executes resource-heavy disk-write operations for downloaded media files.
* **Dispatcher (Orchestrator):** Acts as a middleware layer between the Bot and the Crawler. It intercepts requests, retrieves the best available proxy from the cache, and delegates the processing task. Subsequently, it stores the parsed results in JSONB format.
* **Crawler (Worker):** The service responsible for executing HTTP scraping requests. It uses spoofing techniques (User-Agent spoofing, SOCKS5 tunneling) and applies regular expressions to efficiently parse HTML source code.
* **Database:** A PostgreSQL container used to store action history, available proxies, and links extracted from web pages.
* **VPN Proxy:** An internal SOCKS5 service used to mask the application's IP address during the web scraping process.

## Design Patterns Used

* **Strategy Pattern:**
  - **In the Bot module:** Used to decouple message routing from execution. The `ProcesatorStrategie` interface allows running specific actions without relying on a massive network of if/else statements. Once the decision is made, the system simply calls the polymorphic `Executa()` method.
  - **In the Crawler module:** Through the `IProxy` interface, code can execute HTTP fetches using either a direct or masked connection, without the core web scraping business logic being aware of the underlying network implementation.
* **Simple Factory:**
  - In the Bot, `ComandaBuilder` acts as a factory that analyzes user input text and returns the correct execution instance.
  - In the Crawler, the `CreazaProxyConcret` method evaluates data received over the network and instantiates the correct network client type (`SocksProxy` or `NoProxy`) at runtime.
* **Singleton Pattern:**
  - Used in the implementation of the `ProxyManager` struct within the Dispatcher. Utilizing the `sync.Once` primitive, the system guarantees that the proxy list is queried and loaded from the database into RAM only once upon service startup, remaining globally and safely accessible afterwards.

## Key Technical Concepts

* **Memory Management (Stream I/O):** To prevent excessive RAM consumption when downloading very large videos, data is streamed directly from the network interface to non-volatile memory using the standard `io.Copy` package. Thus, transfer occurs through small buffers, maintaining constant memory consumption.
* **Concurrency and Thread Safety:** The project leverages Goroutines to process messages from multiple users simultaneously. To prevent Race Conditions when accessing shared global resources, state is protected with `sync.Mutex` (for proxy lists) and `sync.RWMutex` (for the paginated link cache).
* **Inter-Process Communication (OS Child Processes):** Converting video files (.mp4) to audio files (.mp3) is performed by delegating the operation to an external process on the host operating system (FFmpeg), instantiated via the `os/exec` package.
* **Stateless UI:** Telegram menu pagination is accomplished without maintaining state on the server. Contextual information (search ID and page index) is serialized and encapsulated inside Callback button strings (`p|ID|Page`), ensuring zero memory consumption between user queries.

## How to Run the Project

The project is fully containerized using Docker, making installation and execution straightforward.

### 1. Prerequisites
Ensure you have the following utilities installed on your host system:
* Docker
* Docker Compose

### 2. Environment Configuration
In the root directory of the project, set up the required environment variables for the bot. You can export these variables in your terminal or create the necessary environment file:

```bash
export TELEGRAM_TOKEN="token-received-from-botfather"
export WEBHOOK_URL="publicly-exposed-server-url"
```

### 3. Starting the System
Open a terminal in the location of your `docker-compose.yml` file and run the build and start command. This command will start the database, execute the table initialization script, and launch all microservices in the network in the background:

```bash
docker compose up --build -d
```

### 4. Stopping the System
To stop and remove the created containers and networks, run:

```bash
docker compose down
```
