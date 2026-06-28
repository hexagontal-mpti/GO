# main.go

**Path:** `p/2026.06.25/gog/main.go`

---

## English 🇬🇧

Asynchronous Telegram bot with an interactive CLI control panel. Polls the Telegram Bot API every second via `getUpdates`, handles incoming messages, and provides a real-time console interface to send replies and view stats.

### Run

```zsh
export TELEGRAM_BOT_TOKEN="your_token"
go run main.go
```

If the environment variable is unset, the program prompts for a token interactively.

### CLI Commands

| Command          | Description                              |
| ---------------- | ---------------------------------------- |
| `stats`          | Show message count, last chat, last offset |
| `send <text>`    | Send a message to the last active chat   |
| `quit` / `exit`  | Shut down the bot                        |

### Architecture

| Function        | Purpose                                                   |
| --------------- | --------------------------------------------------------- |
| `main()`        | Init, launch bot + CLI goroutines, event loop             |
| `runBot()`      | Poll `getUpdates` every second, push to `updates` channel |
| `runCLI()`      | Read stdin, push commands to `commands` channel           |
| `respond()`     | Auto-reply to messages (`/start` – greeting)              |
| `getUpdates()`  | GET `getUpdates` with offset and timeout                  |
| `sendMessage()` | POST `sendMessage` to Telegram API                        |

### Data Structures

`Update`, `Message`, `Chat`, `User` — direct mapping of Telegram API JSON responses. `apiResponse[T]` — generic wrapper for all API responses.

---

## Русский 🇷🇺

Асинхронный Telegram-бот с интерактивной CLI-панелью управления. Бот опрашивает Telegram Bot API каждую секунду через `getUpdates`, обрабатывает входящие сообщения и предоставляет консольный интерфейс для отправки ответов и просмотра статистики.

### Запуск

```zsh
export TELEGRAM_BOT_TOKEN="ваш_токен"
go run main.go
```

Если переменная окружения не задана, программа предложит ввести токен вручную.

### CLI-команды

| Команда          | Описание                                      |
| ---------------- | --------------------------------------------- |
| `stats`          | Статистика: кол-во сообщений, последний чат   |
| `send <text>`    | Отправить текст в последний активный чат      |
| `quit` / `exit`  | Завершить работу бота                         |

### Архитектура

| Функция          | Назначение                                                    |
| ---------------- | ------------------------------------------------------------- |
| `main()`         | Инициализация, запуск горутин bot + CLI, обработка событий    |
| `runBot()`       | Опрос `getUpdates` каждую секунду, отправка в канал `updates` |
| `runCLI()`       | Чтение stdin, отправка команд в канал `commands`              |
| `respond()`      | Авто-ответ на сообщения (`/start` — приветствие)              |
| `getUpdates()`   | GET-запрос к Telegram API с offset и timeout                  |
| `sendMessage()`  | POST-запрос `sendMessage`                                     |

### Структуры данных

`Update`, `Message`, `Chat`, `User` — маппинг JSON-ответов Telegram API. `apiResponse[T]` — дженерик-обёртка для всех ответов API.

---

## Español 🇪🇸

Bot de Telegram asíncrono con un panel de control CLI interactivo. Consulta la API de Telegram cada segundo mediante `getUpdates`, procesa los mensajes entrantes y proporciona una interfaz de consola para enviar respuestas y ver estadísticas.

### Ejecutar

```zsh
export TELEGRAM_BOT_TOKEN="tu_token"
go run main.go
```

Si la variable de entorno no está definida, el programa solicitará el token de forma interactiva.

### Comandos CLI

| Comando          | Descripción                                 |
| ---------------- | ------------------------------------------- |
| `stats`          | Ver estadísticas: mensajes, último chat     |
| `send <text>`    | Enviar un mensaje al último chat activo     |
| `quit` / `exit`  | Apagar el bot                               |

### Arquitectura

| Función          | Propósito                                                    |
| ---------------- | ------------------------------------------------------------ |
| `main()`         | Inicialización, lanzar gorutinas bot + CLI, bucle de eventos |
| `runBot()`       | Consultar `getUpdates` cada segundo, enviar al canal `updates` |
| `runCLI()`       | Leer stdin, enviar comandos al canal `commands`              |
| `respond()`      | Responder automáticamente (`/start` — saludo)                |
| `getUpdates()`   | Solicitud GET a Telegram API con offset y timeout            |
| `sendMessage()`  | Solicitud POST `sendMessage`                                 |

### Estructuras de datos

`Update`, `Message`, `Chat`, `User` — mapeo directo de las respuestas JSON de la API de Telegram. `apiResponse[T]` — envoltorio genérico para todas las respuestas de la API.
