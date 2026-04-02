# Туннель на своём сервере (frp + Traefik + reg.ru)

Самостоятельный аналог **ngrok** / **Cloudflare Tunnel** для домена **limpopo113.ru**: клиент **frpc** подключается к вашему серверу, а **Traefik** выдаёт **HTTPS** (в том числе **wildcard** `*.limpopo113.ru`) через **DNS-01** у регистратора **reg.ru**.

## Что внутри

| Компонент | Роль |
|-----------|------|
| **frps** | Приём туннелей от frpc, маршрутизация HTTP по поддомену |
| **Traefik** | TLS на 443, сертификаты Let’s Encrypt, провайдер DNS **regru** |
| **frpc** (на вашем ПК) | Проброс `localhost` на выбранный поддомен |

Поддомен задаётся **на клиенте** в `frpc.toml`: либо `subdomain = "имя"` (если на сервере задан `subDomainHost = "limpopo113.ru"`), либо `customDomains = ["имя.limpopo113.ru"]`.

## DNS в reg.ru

На IP сервера (например `217.114.1.173`):

- Запись **A** для `@` → IP сервера.
- Запись **A** для **`*`** (wildcard) → тот же IP.

Без wildcard-сертификат и маршрутизация по `*.limpopo113.ru` всё равно возможны для явно перечисленных имён, но удобнее сразу настроить `*`.

## API reg.ru для Traefik

Для **DNS challenge** Traefik использует учётные данные API reg.ru (переменные `REGRU_USERNAME` / `REGRU_PASSWORD`). В кабинете reg.ru может потребоваться **отдельный пароль API** — смотрите документацию reg.ru для доступа по API.

## Подготовка на сервере

- Установлены **Docker** и плагин **Compose v2**.
- Открыты порты **80**, **443** и **7000** (tcp) на фаерволе.

## Конфигурация

1. Скопируйте пример окружения и заполните **без коммита в git**:

   ```bash
   cp .env.example .env
   ```

   В `.env` укажите как минимум:

   - `ACME_EMAIL` — почта для Let’s Encrypt;
   - `REGRU_USERNAME`, `REGRU_PASSWORD` — доступ API reg.ru;
   - `REGRU_PROPAGATION_TIMEOUT`, `REGRU_POLLING_INTERVAL` — при медленном DNS увеличьте таймаут (например 300 и 5);
   - `FRP_TOKEN` — длинный случайный секрет; **тот же** токен будет в `frpc.toml` на клиентах.

2. Локально сгенерируйте `frps.toml` (или сделает это `deploy.sh` перед копированием):

   ```bash
   python3 scripts/generate_frps_toml.py
   ```

   Либо скопируйте `frps.toml.example` → `frps.toml` и вручную пропишите `auth.token`.

## Деплой на сервер по SSH

Нужны **bash**, **ssh**, **scp**, **python3** и клиент **Docker Compose** на удалённой машине. На Windows удобно **Git Bash** или **WSL**.

```bash
chmod +x deploy.sh
./deploy.sh
```

По умолчанию: `root@217.114.1.173`, каталог **`~/tunnel`** на сервере (реальный путь вроде `/root/tunnel` для `root`). Другой хост или путь:

```bash
SERVER=root@другой-хост REMOTE_PATH=/opt/tunnel ./deploy.sh
```

Скрипт: создаёт при необходимости `letsencrypt/acme.json` с правами `600`, копирует `docker-compose.yml`, `.env`, `frps.toml`, выполняет `docker compose pull && docker compose up -d`.

## Запуск своего туннеля (клиент)

1. Скачайте **frpc** с [релизов frp](https://github.com/fatedier/frp/releases) под вашу ОС.

2. Создайте `frpc.toml` по образцу `examples/frpc.toml`:

   - `serverAddr = "limpopo113.ru"`
   - `serverPort = 7000`
   - `auth.token` — **как в** `frps.toml` / `FRP_TOKEN` в `.env`
   - для сайта на `http://127.0.0.1:3000`:

     ```toml
     [[proxies]]
     name = "my-app"
     type = "http"
     localIP = "127.0.0.1"
     localPort = 3000
     subdomain = "my-app"
     ```

   В браузере: `https://my-app.limpopo113.ru`.

3. Альтернатива — явное имя:

   ```toml
   customDomains = ["demo.limpopo113.ru"]
   ```

   (без строки `subdomain` для этого прокси.)

4. Запуск клиента:

   ```bash
   frpc -c frpc.toml
   ```

Первый выпуск сертификата Let’s Encrypt через DNS может занять несколько минут; при ошибках увеличьте `REGRU_PROPAGATION_TIMEOUT` и смотрите логи Traefik: `docker compose logs -f traefik`.

## Безопасность

- **Не публикуйте** `.env` и `frps.toml` с токеном.
- Пароли и токены, которые когда-либо отправляли в открытый чат, лучше **сменить** (в том числе пароль API reg.ru и `FRP_TOKEN`).

## Другие варианты «как ngrok на своём сервере»

Кроме **frp**, распространены **inlets**, **boringproxy**, **zrok** (свой relay), **Tailscale Funnel** — у каждого свои плюсы; здесь выбран frp как простой и предсказуемый HTTP-туннель с явным поддоменом на клиенте.
