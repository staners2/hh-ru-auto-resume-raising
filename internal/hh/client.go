// client.go
package hh

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type Client struct {
	Username  string
	Password  string
	UserAgent string
	xsrf      string
	hhtoken   string
	client    *http.Client
}

type Resume struct {
	ID    string
	Title string
}

func NewClient(login, password, proxy string) (*Client, error) {
	client := &Client{
		Username:  login,
		Password:  password,
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	}

	if proxy != "None" && proxy != "" {
		proxyURL, _ := url.Parse(proxy)
		client.client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}
	} else {
		client.client = &http.Client{}
	}

	return client, nil
}

func (c *Client) getCookieAnonymous() error {
	log.Println("Making HEAD request to hh.ru to get anonymous cookies...")
	req, _ := http.NewRequest("HEAD", "https://hh.ru/", nil)
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Error making request to hh.ru: %v", err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("Response status: %s", resp.Status)

	// Преобразуем заголовки в строку как в Python: str(response.headers)
	headersStr := fmt.Sprintf("%v", resp.Header)
	log.Printf("Response headers string: %s", headersStr)

	// Ищем токены используя точно такие же regex как в Python
	xsrfRegex := regexp.MustCompile(`_xsrf=([^;]+);`)
	if xsrfMatch := xsrfRegex.FindStringSubmatch(headersStr); len(xsrfMatch) > 1 {
		c.xsrf = xsrfMatch[1]
		log.Printf("Found XSRF token: %s", c.xsrf[:min(8, len(c.xsrf))]+"...")
	} else {
		log.Printf("XSRF token not found in headers")
		return fmt.Errorf("XSRF token not found")
	}

	hhtokenRegex := regexp.MustCompile(`hhtoken=([^;]+);`)
	if hhtokenMatch := hhtokenRegex.FindStringSubmatch(headersStr); len(hhtokenMatch) > 1 {
		c.hhtoken = hhtokenMatch[1]
		log.Printf("Found HH token: %s", c.hhtoken[:min(8, len(c.hhtoken))]+"...")
	} else {
		log.Printf("HH token not found in headers")
		return fmt.Errorf("HH token not found")
	}

	log.Printf("Successfully extracted both tokens from anonymous request")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *Client) Login() error {
	// Получаем анонимные куки точно как в Python
	if err := c.getCookieAnonymous(); err != nil {
		return err
	}

	// Используем Go стандартную библиотеку multipart, но с кастомным boundary
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// Устанавливаем boundary точно как в Python
	boundary := "boundary"
	writer.SetBoundary(boundary)

	// Формируем данные точно как в Python
	_ = writer.WriteField("_xsrf", c.xsrf)
	_ = writer.WriteField("backUrl", "https://hh.ru/")
	_ = writer.WriteField("failUrl", "/account/login")
	_ = writer.WriteField("remember", "yes")
	_ = writer.WriteField("username", c.Username)
	_ = writer.WriteField("password", c.Password)
	_ = writer.WriteField("username", c.Username) // Дублируем username как в Python
	_ = writer.WriteField("isBot", "false")
	_ = writer.Close()

	log.Printf("Attempting login for user: %s", c.Username)
	log.Printf("POST data length: %d bytes", buf.Len())
	log.Printf("Boundary: %s", boundary)
	
	req, _ := http.NewRequest("POST", "https://hh.ru/account/login", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Cookie", fmt.Sprintf("_xsrf=%s; hhtoken=%s;", c.xsrf, c.hhtoken))
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("X-Xsrftoken", c.xsrf)

	// Логируем все заголовки запроса
	log.Println("=== LOGIN REQUEST HEADERS ===")
	for name, values := range req.Header {
		for _, value := range values {
			log.Printf("%s: %s", name, value)
		}
	}
	log.Println("=== END HEADERS ===")
	
	// Логируем тело запроса (без пароля)
	sanitizedData := strings.ReplaceAll(buf.String(), c.Password, "***HIDDEN***")
	log.Printf("=== LOGIN REQUEST BODY ===\n%s\n=== END BODY ===", sanitizedData)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Login request failed: %v", err)
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("Login response status: %s", resp.Status)

	// Извлекаем токены из ответа точно как в Python
	allCookies := resp.Header["Set-Cookie"]
	cookiesStr := strings.Join(allCookies, "; ")
	log.Printf("Login response cookies: %v", allCookies)

	// Обновляем XSRF токен
	if xsrfMatch := regexp.MustCompile(`_xsrf=([^;]+)`).FindStringSubmatch(cookiesStr); len(xsrfMatch) > 1 {
		c.xsrf = xsrfMatch[1]
		log.Printf("Updated XSRF token from login response")
	}

	// Проверяем наличие hhtoken для подтверждения успешной авторизации
	if hhtokenMatch := regexp.MustCompile(`hhtoken=([^;]+)`).FindStringSubmatch(cookiesStr); len(hhtokenMatch) > 1 {
		c.hhtoken = hhtokenMatch[1]
		log.Printf("Got HH token from login response")
		return nil
	}

	log.Printf("Login failed - no hhtoken found in response")
	return fmt.Errorf("failed to get authentication tokens (status: %s)", resp.Status)
}

func (c *Client) GetResumes() ([]Resume, error) {
	req, _ := http.NewRequest("GET", "https://hh.ru/applicant/resumes", nil)
	req.Header.Set("Cookie", fmt.Sprintf("_xsrf=%s; hhtoken=%s;", c.xsrf, c.hhtoken))
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get resumes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	content := string(body)
	
	// Парсим резюме из HTML используя regex как в Python версии
	// Ищем элементы с data-qa="resume" и извлекаем название и ID
	resumePattern := regexp.MustCompile(`<div[^>]*data-qa="resume"[^>]*data-qa-title="([^"]+)"[^>]*>[\s\S]*?<a[^>]*href="[^"]*resume/([a-f0-9]+)`)
	matches := resumePattern.FindAllStringSubmatch(content, -1)

	var resumes []Resume
	for _, match := range matches {
		if len(match) > 2 {
			resumes = append(resumes, Resume{
				Title: match[1],
				ID:    match[2],
			})
		}
	}

	log.Printf("Found %d resumes", len(resumes))
	return resumes, nil
}

func (c *Client) RaiseResume(resumeID string) (int, error) {
	// Используем тот же подход что и в Python - multipart.Writer с кастомным boundary
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// Устанавливаем boundary точно как в Python
	boundary := "boundary"
	writer.SetBoundary(boundary)

	// Формируем данные точно как в Python для поднятия резюме
	_ = writer.WriteField("resume", resumeID)
	_ = writer.WriteField("undirectable", "true")
	_ = writer.Close()

	log.Printf("Raising resume with ID: %s", resumeID)
	log.Printf("POST data length: %d bytes", buf.Len())

	req, _ := http.NewRequest("POST", "https://hh.ru/applicant/resumes/touch", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Cookie", fmt.Sprintf("_xsrf=%s; hhtoken=%s;", c.xsrf, c.hhtoken))
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("X-Xsrftoken", c.xsrf)

	// Логируем заголовки для отладки
	log.Println("=== RAISE RESUME HEADERS ===")
	for name, values := range req.Header {
		for _, value := range values {
			if name == "Cookie" {
				log.Printf("%s: _xsrf=***; hhtoken=***;", name)
			} else {
				log.Printf("%s: %s", name, value)
			}
		}
	}
	log.Println("=== END HEADERS ===")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("Raise resume request failed: %v", err)
		return 0, fmt.Errorf("failed to raise resume: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("Raise resume response status: %s", resp.Status)
	
	// Читаем тело ответа для дополнительной информации при ошибках
	if resp.StatusCode != 200 && resp.StatusCode != 409 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Raise resume response body: %s", string(body)[:min(500, len(string(body)))])
	}

	return resp.StatusCode, nil
}

func (c *Client) GetTokens() (string, string) {
	return c.xsrf, c.hhtoken
}

func (c *Client) SetTokens(xsrf, hhtoken string) {
	c.xsrf = xsrf
	c.hhtoken = hhtoken
}
