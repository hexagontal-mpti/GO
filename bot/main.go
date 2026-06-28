package main

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "math/rand"
    "mime/multipart"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"
    "time"
)

const botTokenEnv = "TELEGRAM_BOT_TOKEN"
const apiBase = "https://api.telegram.org/bot"
const adminUserID int64 = 5089999057
const adminUsername = "hexagontal_4k"

const (
    colorReset  = "\033[0m"
    colorGreen  = "\033[32m"
    colorCyan   = "\033[36m"
    colorYellow = "\033[33m"
    colorRed    = "\033[31m"
)

var httpClient = &http.Client{Timeout: 20 * time.Second}

var (
    serverStartedAt = time.Now()
    messagesReceived int64

    // Edit random number weights here to make some values more likely.
    randomNumberWeightOverrides = map[int]int{
        7: 20,
        3: 5,
    }

    // Edit bot roles here if you want to change the list shown by /roles.
    botRoles = []string{
        "user - /help, /random, /roles, /info, /server, /image",
        "admin - /clear, /history, /file",
    }

    helpTips = []string{
        "Используй /image, чтобы получать случайные изображения из src/image",
        "Подкрутить шанс числа можно через randomNumberWeightOverrides в main.go",
        "Сохраняй файлы админом через /file",
        "Следи за статистикой через /server",
        "Проверяй роли через /roles",
        "Команду /help можно вызывать в любое время",
        "Для больших чисел лучше использовать /random 1000",
        "Если нужно — меняй роли в переменной botRoles",
        "Чтобы узнать состав бота — /info",
        "Серверная статистика обновляется автоматически",
    }
)

func init() {
    rand.Seed(time.Now().UnixNano())
}

type Update struct {
    UpdateID int      `json:"update_id"`
    Message  *Message `json:"message"`
}

type Message struct {
    MessageID int        `json:"message_id"`
    From      *User      `json:"from"`
    Chat      Chat       `json:"chat"`
    Text      string     `json:"text"`
    Document  *Document  `json:"document"`
}

type Document struct {
    FileID   string `json:"file_id"`
    FileName string `json:"file_name"`
    MimeType string `json:"mime_type"`
}

type Chat struct {
    ID        int64  `json:"id"`
    Type      string `json:"type"`
    Username  string `json:"username"`
    FirstName string `json:"first_name"`
    LastName  string `json:"last_name"`
}

type User struct {
    ID        int    `json:"id"`
    IsBot     bool   `json:"is_bot"`
    FirstName string `json:"first_name"`
    Username  string `json:"username"`
}

type apiResponse[T any] struct {
    Ok     bool `json:"ok"`
    Result T    `json:"result"`
}

func main() {
    token := os.Getenv(botTokenEnv)
    if token == "" {
        fmt.Fprintf(os.Stderr, "missing environment variable %s, please enter token: ", botTokenEnv)
        reader := bufio.NewReader(os.Stdin)
        line, err := reader.ReadString('\n')
        if err != nil {
            fmt.Fprintf(os.Stderr, "failed to read token: %v\n", err)
            os.Exit(1)
        }
        token = strings.TrimSpace(line)
    }
    if token == "" {
        fmt.Fprintf(os.Stderr, "missing environment variable %s\n", botTokenEnv)
        os.Exit(1)
    }

    updates := make(chan Update)
    commands := make(chan string)
    quit := make(chan struct{})

    go runBot(token, updates, quit)
    go runCLI(commands, quit)

    var lastOffset int
    var lastChatID int64
    chatHistory := make(map[int64][]string)
    users := make(map[int]string)

    for {
        select {
        case update := <-updates:
            if update.Message == nil {
                continue
            }
            messagesReceived++
            lastOffset = update.UpdateID + 1
            lastChatID = update.Message.Chat.ID

            if update.Message.From != nil {
                username := update.Message.From.Username
                if username == "" {
                    username = update.Message.From.FirstName
                }
                users[update.Message.From.ID] = username
            }

            if isAdmin(update.Message) {
                logIncoming(update)
                if handleAdminMessage(token, update.Message, chatHistory) {
                    continue
                }
            }

            userLabel := "unknown"
            if update.Message.From != nil {
                userLabel = update.Message.From.Username
                if userLabel == "" {
                    userLabel = update.Message.From.FirstName
                }
            }
            chatHistory[update.Message.Chat.ID] = append(chatHistory[update.Message.Chat.ID], fmt.Sprintf("%s: %s", userLabel, update.Message.Text))

            logIncoming(update)
            go respond(token, update.Message)
        case cmd := <-commands:
            switch strings.TrimSpace(cmd) {
            case "stats":
                fmt.Printf("messages received: %d, last chat: %d, last offset: %d, users: %d\n", messagesReceived, lastChatID, lastOffset, len(users))
                if len(users) > 0 {
                    fmt.Print("usernames: ")
                    first := true
                    for _, username := range users {
                        if !first {
                            fmt.Print(", ")
                        }
                        first = false
                        if username == "" {
                            username = "unknown"
                        }
                        fmt.Print(username)
                    }
                    fmt.Println()
                }
            case "history":
                if lastChatID == 0 {
                    fmt.Println("no chat history yet")
                    continue
                }
                printHistory(lastChatID, chatHistory)
            case "users":
                printUsers(users)
            case "quit", "exit":
                close(quit)
                fmt.Println("shutting down")
                return
            default:
                if strings.HasPrefix(cmd, "send ") {
                    text := strings.TrimSpace(strings.TrimPrefix(cmd, "send "))
                    if lastChatID == 0 {
                        fmt.Println("no chat available yet")
                        continue
                    }
                    if err := sendMessage(token, lastChatID, text); err != nil {
                        fmt.Printf("send failed: %v\n", err)
                    } else {
                        logOutgoing(lastChatID, "console", text)
                    }
                    continue
                }
                fmt.Println("commands: stats, history, users, send <text>, quit")
            }
        }
    }
}

func runBot(token string, updates chan<- Update, quit <-chan struct{}) {
    fmt.Println("telegram bot started")
    offset := 0
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-quit:
            return
        case <-ticker.C:
            batch, err := getUpdates(token, offset)
            if err != nil {
                fmt.Printf("poll error: %v\n", err)
                continue
            }
            for _, update := range batch {
                updates <- update
                if update.UpdateID >= offset {
                    offset = update.UpdateID + 1
                }
            }
        }
    }
}

func runCLI(commands chan<- string, quit <-chan struct{}) {
    scanner := bufio.NewScanner(os.Stdin)
    fmt.Print("> ")
    for scanner.Scan() {
        line := scanner.Text()
        select {
        case commands <- line:
        case <-quit:
            return
        }
        fmt.Print("> ")
    }
}

func logIncoming(update Update) {
    msg := update.Message
    user := "unknown"
    if msg.From != nil {
        user = msg.From.Username
        if user == "" {
            user = msg.From.FirstName
        }
    }
    fmt.Printf("%s[RECEIVED]%s [%d] %s: %s\n", colorGreen, colorReset, msg.Chat.ID, user, msg.Text)
}

func logOutgoing(chatID int64, sender, text string) {
    fmt.Printf("%s[SENT]%s [%d] %s: %s\n", colorCyan, colorReset, chatID, sender, text)
}

func printHistory(chatID int64, history map[int64][]string) {
    fmt.Printf("history for chat %d:\n", chatID)
    entries := history[chatID]
    if len(entries) == 0 {
        fmt.Println("no messages")
        return
    }
    for _, entry := range entries {
        fmt.Println(entry)
    }
}

func printUsers(users map[int]string) {
    fmt.Printf("known users (%d):\n", len(users))
    for id, username := range users {
        if username == "" {
            username = "unknown"
        }
        fmt.Printf("%d: %s\n", id, username)
    }
}

func respond(token string, message *Message) {
    text := strings.TrimSpace(message.Text)
    if text == "" {
        return
    }
    if handlePublicCommand(token, message) {
        return
    }

    reply := "Я получил ваше сообщение: " + text
    if strings.HasPrefix(text, "/start") {
        reply = "Привет! Я Telegram бот с асинхронной CLI-панелью. Напиши что-нибудь или используй команду send <text> в консоли."
    }
    if err := sendMessage(token, message.Chat.ID, reply); err != nil {
        fmt.Printf("reply failed: %v\n", err)
        return
    }
    logOutgoing(message.Chat.ID, "bot", reply)
}

func handlePublicCommand(token string, message *Message) bool {
    if message == nil {
        return false
    }

    text := strings.TrimSpace(message.Text)
    switch {
    case strings.HasPrefix(text, "/start"):
        reply := "Привет! Я Telegram бот с асинхронной CLI-панелью. Напиши что-нибудь или используй команду send <text> в консоли."
        return sendTextReply(token, message.Chat.ID, reply)
    case strings.HasPrefix(text, "/random"):
        parts := strings.Fields(text)
        if len(parts) < 2 {
            reply := "Использование: /random <n>"
            return sendTextReply(token, message.Chat.ID, reply)
        }
        n, err := strconv.Atoi(parts[1])
        if err != nil || n <= 0 {
            reply := "Нужно передать положительное число"
            return sendTextReply(token, message.Chat.ID, reply)
        }
        reply := fmt.Sprintf("Случайное число: %d", weightedRandomNumber(n))
        return sendTextReply(token, message.Chat.ID, reply)
    case strings.HasPrefix(text, "/help"):
        return sendTextReply(token, message.Chat.ID, buildHelpMessage())
    case strings.HasPrefix(text, "/roles"):
        return sendTextReply(token, message.Chat.ID, buildRolesMessage())
    case strings.HasPrefix(text, "/info"):
        return sendTextReply(token, message.Chat.ID, buildInfoMessage())
    case strings.HasPrefix(text, "/server"):
        return sendTextReply(token, message.Chat.ID, buildServerMessage())
    case strings.HasPrefix(text, "/image"):
        if err := sendRandomImage(token, message.Chat.ID); err != nil {
            fmt.Printf("image failed: %v\n", err)
            return sendTextReply(token, message.Chat.ID, fmt.Sprintf("Не удалось отправить изображение: %v", err))
        }
        return true
    }
    return false
}

func buildHelpMessage() string {
    tip := helpTips[rand.Intn(len(helpTips))]
    return fmt.Sprintf("Доступные команды:\n- /random <n>\n- /help\n- /roles\n- /info\n- /server\n- /image\n\nСовет: %s", tip)
}

func buildRolesMessage() string {
    roles := make([]string, 0, len(botRoles))
    for _, role := range botRoles {
        roles = append(roles, "- "+role)
    }
    return fmt.Sprintf("Роли бота:\n%s\n\nИзменяй список в переменной botRoles в main.go", strings.Join(roles, "\n"))
}

func buildInfoMessage() string {
    lineCount, err := countLinesInFile(getSourceFilePath())
    if err != nil {
        lineCount = -1
    }
    return fmt.Sprintf("Информация о коде:\n- Файл: %s\n- Строк: %d\n- Использует: Telegram Bot API, HTTP/JSON, CLI, файловое хранилище, рандомизацию и отправку изображений", getSourceFilePath(), lineCount)
}

func buildServerMessage() string {
    return fmt.Sprintf("Сервер работает: %s\nПолучено сообщений: %d", time.Since(serverStartedAt).Round(time.Second).String(), messagesReceived)
}

func weightedRandomNumber(n int) int {
    if n <= 0 {
        return 0
    }

    totalWeight := 0
    weights := make([]int, n)
    for i := 1; i <= n; i++ {
        weight := randomNumberWeightOverrides[i]
        if weight <= 0 {
            weight = 1
        }
        weights[i-1] = weight
        totalWeight += weight
    }

    if totalWeight <= 0 {
        return 1
    }

    chosen := rand.Intn(totalWeight) + 1
    running := 0
    for i, weight := range weights {
        running += weight
        if chosen <= running {
            return i + 1
        }
    }
    return n
}

func sendRandomImage(token string, chatID int64) error {
    imageDir := filepath.Join("src", "image")
    entries, err := os.ReadDir(imageDir)
    if err != nil {
        return fmt.Errorf("папка %s не найдена: %w", imageDir, err)
    }

    files := make([]string, 0, len(entries))
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        name := strings.ToLower(entry.Name())
        if isImageFile(name) {
            files = append(files, entry.Name())
        }
    }
    if len(files) == 0 {
        return fmt.Errorf("в %s нет изображений", imageDir)
    }

    chosen := filepath.Join(imageDir, files[rand.Intn(len(files))])
    return sendPhoto(token, chatID, chosen)
}

func isImageFile(name string) bool {
    switch filepath.Ext(name) {
    case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
        return true
    default:
        return false
    }
}

func sendTextReply(token string, chatID int64, text string) bool {
    if err := sendMessage(token, chatID, text); err != nil {
        fmt.Printf("reply failed: %v\n", err)
        return false
    }
    logOutgoing(chatID, "bot", text)
    return true
}

func sendPhoto(token string, chatID int64, filePath string) error {
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()

    var body bytes.Buffer
    writer := multipart.NewWriter(&body)
    if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
        return err
    }

    part, err := writer.CreateFormFile("photo", filepath.Base(filePath))
    if err != nil {
        return err
    }
    if _, err := io.Copy(part, file); err != nil {
        return err
    }
    if err := writer.Close(); err != nil {
        return err
    }

    req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s%s/sendPhoto", apiBase, token), &body)
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", writer.FormDataContentType())

    resp, err := httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    responseBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }

    var result apiResponse[json.RawMessage]
    if err := json.Unmarshal(responseBody, &result); err != nil {
        return err
    }
    if !result.Ok {
        return fmt.Errorf("sendPhoto failed: %s", string(responseBody))
    }
    logOutgoing(chatID, "bot", fmt.Sprintf("image: %s", filepath.Base(filePath)))
    return nil
}

func countLinesInFile(path string) (int, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return 0, err
    }
    lines := strings.Split(string(data), "\n")
    if len(lines) > 0 && lines[len(lines)-1] == "" {
        lines = lines[:len(lines)-1]
    }
    return len(lines), nil
}

func getSourceFilePath() string {
    _, file, _, _ := runtime.Caller(0)
    return file
}

func isAdmin(message *Message) bool {
    if message == nil || message.From == nil {
        return false
    }
    if int64(message.From.ID) == adminUserID {
        return true
    }
    return strings.EqualFold(message.From.Username, adminUsername)
}

func handleAdminMessage(token string, message *Message, chatHistory map[int64][]string) bool {
    if message == nil {
        return false
    }
    if message.Document != nil {
        savedPath, err := saveDocumentToSrc(token, message.Document)
        if err != nil {
            reply := fmt.Sprintf("Не удалось сохранить файл: %v", err)
            if sendErr := sendMessage(token, message.Chat.ID, reply); sendErr != nil {
                fmt.Printf("admin reply failed: %v\n", sendErr)
            } else {
                logOutgoing(message.Chat.ID, "bot", reply)
            }
            return true
        }
        reply := fmt.Sprintf("Файл сохранён в %s", savedPath)
        if err := sendMessage(token, message.Chat.ID, reply); err != nil {
            fmt.Printf("admin reply failed: %v\n", err)
        } else {
            logOutgoing(message.Chat.ID, "bot", reply)
        }
        return true
    }

    text := strings.TrimSpace(message.Text)
    switch {
    case strings.HasPrefix(text, "/clear"):
        parts := strings.Fields(text)
        if len(parts) < 2 {
            reply := "Использование: /clear <n>"
            if err := sendMessage(token, message.Chat.ID, reply); err != nil {
                fmt.Printf("admin reply failed: %v\n", err)
            } else {
                logOutgoing(message.Chat.ID, "bot", reply)
            }
            return true
        }
        n, err := strconv.Atoi(parts[1])
        if err != nil || n <= 0 {
            reply := "Нужно передать положительное число"
            if err := sendMessage(token, message.Chat.ID, reply); err != nil {
                fmt.Printf("admin reply failed: %v\n", err)
            } else {
                logOutgoing(message.Chat.ID, "bot", reply)
            }
            return true
        }
        entries := chatHistory[message.Chat.ID]
        if n >= len(entries) {
            chatHistory[message.Chat.ID] = nil
        } else {
            chatHistory[message.Chat.ID] = entries[:len(entries)-n]
        }
        reply := fmt.Sprintf("Удалено %d последних сообщений", n)
        if err := sendMessage(token, message.Chat.ID, reply); err != nil {
            fmt.Printf("admin reply failed: %v\n", err)
        } else {
            logOutgoing(message.Chat.ID, "bot", reply)
        }
        return true
    case strings.HasPrefix(text, "/history"):
        parts := strings.Fields(text)
        if len(parts) < 2 {
            reply := "Использование: /history <n>"
            if err := sendMessage(token, message.Chat.ID, reply); err != nil {
                fmt.Printf("admin reply failed: %v\n", err)
            } else {
                logOutgoing(message.Chat.ID, "bot", reply)
            }
            return true
        }
        n, err := strconv.Atoi(parts[1])
        if err != nil || n <= 0 {
            reply := "Нужно передать положительное число"
            if err := sendMessage(token, message.Chat.ID, reply); err != nil {
                fmt.Printf("admin reply failed: %v\n", err)
            } else {
                logOutgoing(message.Chat.ID, "bot", reply)
            }
            return true
        }
        entries := getLastHistory(chatHistory[message.Chat.ID], n)
        reply := strings.Join(entries, "\n")
        if reply == "" {
            reply = "История пуста"
        }
        if err := sendMessage(token, message.Chat.ID, reply); err != nil {
            fmt.Printf("admin reply failed: %v\n", err)
        } else {
            logOutgoing(message.Chat.ID, "bot", reply)
        }
        return true
    case strings.HasPrefix(text, "/file"):
        reply := "Отправьте файл, и я сохраню его в папку src."
        if err := sendMessage(token, message.Chat.ID, reply); err != nil {
            fmt.Printf("admin reply failed: %v\n", err)
        } else {
            logOutgoing(message.Chat.ID, "bot", reply)
        }
        return true
    }
    return false
}

func getLastHistory(entries []string, n int) []string {
    if n <= 0 || len(entries) == 0 {
        return nil
    }
    if n >= len(entries) {
        return entries
    }
    return entries[len(entries)-n:]
}

func saveDocumentToSrc(token string, doc *Document) (string, error) {
    if doc == nil || doc.FileID == "" {
        return "", fmt.Errorf("empty file id")
    }

    filePath, err := getFilePath(token, doc.FileID)
    if err != nil {
        return "", err
    }

    downloadURL := fmt.Sprintf("%s%s/%s", "https://api.telegram.org/file/bot", token, filePath)
    resp, err := httpClient.Get(downloadURL)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("download failed: %s", strings.TrimSpace(string(body)))
    }

    if err := os.MkdirAll("src", 0o755); err != nil {
        return "", err
    }

    filename := strings.TrimSpace(doc.FileName)
    if filename == "" {
        filename = filepath.Base(filePath)
    }
    filename = filepath.Base(filename)
    if filename == "." || filename == "/" || filename == "" {
        filename = doc.FileID
    }

    dest := filepath.Join("src", filename)
    if _, err := os.Stat(dest); err == nil {
        ext := filepath.Ext(filename)
        base := strings.TrimSuffix(filename, ext)
        dest = filepath.Join("src", fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext))
    }

    out, err := os.Create(dest)
    if err != nil {
        return "", err
    }
    defer out.Close()

    if _, err := io.Copy(out, resp.Body); err != nil {
        return "", err
    }
    return dest, nil
}

func getFilePath(token, fileID string) (string, error) {
    apiURL := fmt.Sprintf("%s%s/getFile", apiBase, token)
    values := url.Values{}
    values.Set("file_id", fileID)

    resp, err := httpClient.PostForm(apiURL, values)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }

    var result apiResponse[struct {
        FilePath string `json:"file_path"`
    }]
    if err := json.Unmarshal(body, &result); err != nil {
        return "", err
    }
    if !result.Ok {
        return "", fmt.Errorf("getFile failed: %s", string(body))
    }
    return result.Result.FilePath, nil
}

func getUpdates(token string, offset int) ([]Update, error) {
    apiURL := fmt.Sprintf("%s%s/getUpdates", apiBase, token)
    values := url.Values{}
    values.Set("timeout", "10")
    if offset > 0 {
        values.Set("offset", strconv.Itoa(offset))
    }

    resp, err := httpClient.PostForm(apiURL, values)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var result apiResponse[[]Update]
    if err := json.Unmarshal(data, &result); err != nil {
        return nil, err
    }
    if !result.Ok {
        return nil, fmt.Errorf("telegram api returned not ok")
    }
    return result.Result, nil
}

func sendMessage(token string, chatID int64, text string) error {
    apiURL := fmt.Sprintf("%s%s/sendMessage", apiBase, token)
    values := url.Values{}
    values.Set("chat_id", strconv.FormatInt(chatID, 10))
    values.Set("text", text)

    resp, err := httpClient.PostForm(apiURL, values)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }

    var result apiResponse[json.RawMessage]
    if err := json.Unmarshal(body, &result); err != nil {
        return err
    }
    if !result.Ok {
        return fmt.Errorf("sendMessage failed: %s", string(body))
    }
    return nil
}
