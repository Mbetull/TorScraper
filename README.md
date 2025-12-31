# TorScraper (Go)

Tor SOCKS5 proxy üzerinden .onion hedeflerini eşzamanlı tarar, erişilebilir sayfaların HTML çıktısını ve ekran görüntüsünü kaydeder, sonuçları loglar.

## Requirements
- Go
- Tor (Windows: Tor Browser açık → 127.0.0.1:9150, Linux: tor service → 127.0.0.1:9050)

## Run
```bash
go mod tidy
go run .
