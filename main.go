package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

// Tenant описывает данные арендатора
type Tenant struct {
	Name        string `json:"name"`
	Apartment   string `json:"apartment"`
	PaymentDate string `json:"paymentDate"` // формат "2006-01-02"
}

// Глобальный список арендаторов (in-memory)
var tenantsList []Tenant

// sendTelegramMessage отправляет Telegram-сообщение через Bot API
func sendTelegramMessage(message string) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID") // общий ID (например, админа)

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	body, _ := json.Marshal(map[string]string{
		"chat_id": chatID,
		"text":    message,
	})

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		resBody, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("telegram error: %s", resBody)
	}

	return nil
}

// checkPaymentDates проходит по всем арендаторам и отправляет уведомление, если срок оплаты
// наступает через 1 день или через 3 дня от текущей даты.
func checkPaymentDates() {
	now := time.Now().Truncate(24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)
	threeDaysLater := now.Add(72 * time.Hour)

	fmt.Println("Проверка дат оплаты на", now.Format("2006-01-02"))

	for _, t := range tenantsList {
		paymentDate, err := time.Parse("2006-01-02", t.PaymentDate)
		if err != nil {
			fmt.Printf("Ошибка парсинга даты для %s: %v\n", t.Name, err)
			continue
		}

		if paymentDate.Equal(threeDaysLater) {
			message := fmt.Sprintf("Привет, %s! Напоминаем, что через 3 дня (%s) нужно оплатить аренду квартиры %s.",
				t.Name, paymentDate.Format("02 January 2006"), t.Apartment)
			if err := sendTelegramMessage(message); err != nil {
				fmt.Printf("Ошибка отправки уведомления (за 3 дня) для %s: %v\n", t.Name, err)
			} else {
				fmt.Printf("Уведомление (за 3 дня) успешно отправлено для %s\n", t.Name)
			}
		} else if paymentDate.Equal(tomorrow) {
			message := fmt.Sprintf("Привет, %s! Напоминаем, что завтра (%s) нужно оплатить аренду квартиры %s.",
				t.Name, paymentDate.Format("02 January 2006"), t.Apartment)
			if err := sendTelegramMessage(message); err != nil {
				fmt.Printf("Ошибка отправки уведомления (за 1 день) для %s: %v\n", t.Name, err)
			} else {
				fmt.Printf("Уведомление (за 1 день) успешно отправлено для %s\n", t.Name)
			}
		}
	}
}

func main() {
	// Загружаем переменные окружения из .env
	if err := godotenv.Load(); err != nil {
		fmt.Println("Ошибка загрузки .env файла:", err)
	}

	// Настройка cron-задачи для проверки дат оплаты
	c := cron.New()
	// Задача запускается каждый день в 09:00 (локальное время)
	_, err := c.AddFunc("0 9 * * *", func() {
		fmt.Println("Запуск проверки дат оплаты в 09:00")
		checkPaymentDates()
	})
	if err != nil {
		fmt.Println("Ошибка настройки cron-задачи:", err)
	}
	c.Start()

	// Настройка HTTP-сервера с помощью Gin
	router := gin.Default()

	// Настройка CORS для фронтенда
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5185",          // для локальной разработки
			"https://eskertu-bot.vercel.app", // ✅ разрешаем фронт с Vercel
		},
		AllowMethods:     []string{"POST", "GET", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		AllowCredentials: true,
	}))

	// Эндпоинт для добавления арендатора
	router.POST("/api/tenants", func(c *gin.Context) {
		var t Tenant
		if err := c.ShouldBindJSON(&t); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные"})
			return
		}

		// Добавляем арендатора в глобальный список
		tenantsList = append(tenantsList, t)

		// Отправляем немедленное уведомление о новом арендаторе (опционально)
		message := fmt.Sprintf("📢 Новый квартирант:\n👤 Имя: %s\n🏠 Квартира: %s\n💰 Дата оплаты: %s",
			t.Name, t.Apartment, t.PaymentDate)
		if err := sendTelegramMessage(message); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка Telegram", "details": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Уведомление отправлено и арендатор добавлен",
			"tenant":  t,
		})
	})

	// Запуск сервера на порту 8080
	router.Run(":8080")
}
