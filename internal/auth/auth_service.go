package auth

import (
	"crypto/aes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	loginURL       = "https://sso.hdu.edu.cn/login"
	baseServiceURL = "https://skl.hdu.edu.cn/api/cas/login"
)

type AuthService struct {
}

func NewAuthService() (*AuthService, error) {
	return &AuthService{}, nil
}

func (s *AuthService) Login(username, password string) (string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", fmt.Errorf("创建 cookie jar 失败: %w", err)
	}
	isolatedClient := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 60 * time.Second,
	}
	stateToken, err := generateStateToken(12)
	if err != nil {
		return "", fmt.Errorf("生成 state token 失败: %w", err)
	}

	log.Println("已生成 State Token:", stateToken)
	serviceURLWithState := fmt.Sprintf("%s?state=%s&index=", baseServiceURL, stateToken)

	// log.Println("步骤 1 & 2: 访问登录页并解析令牌...")
	croyptoKey, execution, fullLoginURL, err := s.fetLoginTokens(isolatedClient, serviceURLWithState)
	if err != nil {
		return "", err
	}

	log.Printf("    - AES Key: %s, Execution Token (前20): %s...\n", croyptoKey, execution[:20])

	// log.Println("步骤 3: 加密密码...")
	encryptedPassword, err := s.encryptPassword(croyptoKey, password)
	if err != nil {
		return "", err
	}
	// log.Println("    - 密码加密成功")

	// log.Println("步骤 4 & 5: 发送登录请求...")
	ticketURL, err := s.postLoginForm(isolatedClient, username, encryptedPassword, croyptoKey, execution, fullLoginURL)
	if err != nil {
		return "", err
	}
	// log.Println("    - 登录成功，已获取Ticket URL")
	// log.Println("步骤 6: 访问Ticket URL换取X-Auth-Token...")
	xAuthToken, err := s.exchangeTicketForToken(isolatedClient, ticketURL, fullLoginURL)
	if err != nil {
		return "", err
	}
	// log.Println("🎉🎉🎉 最终胜利！成功获取 X-Auth-Token！ 🎉🎉🎉")
	return xAuthToken, nil
}

func (s *AuthService) fetLoginTokens(client *http.Client, serviceURL string) (croyptoKey, execution, fullLoginURL string, err error) {
	req, _ := http.NewRequest("GET", loginURL, nil)
	q := req.URL.Query()
	q.Add("service", serviceURL)
	req.URL.RawQuery = q.Encode()

	resp, err := client.Get(req.URL.String())
	if err != nil {
		return "", "", "", fmt.Errorf("访问登录页失败: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("解析HTML失败: %w", err)
	}

	croyptoKey = doc.Find("p#login-croypto").Text()
	execution = doc.Find("#login-page-flowkey").Text()
	fullLoginURL = resp.Request.URL.String()

	if croyptoKey == "" || execution == "" {
		return "", "", "", errors.New("未能从HTML中找到AES密钥或Execution令牌")
	}

	return croyptoKey, execution, fullLoginURL, nil
}

func (s *AuthService) encryptPassword(keyB64, rawPassword string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return "", fmt.Errorf("Base64解码AES密钥失败: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建AES cipher失败: %w", err)
	}

	plaintext := []byte(rawPassword)
	paddedPlaintext := pkcs7Pad(plaintext)
	ciphertext := make([]byte, len(paddedPlaintext))

	mode := newECBEncrypter(block)
	mode.CryptBlocks(ciphertext, paddedPlaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}
func (s *AuthService) postLoginForm(client *http.Client, user, encPass, cryptoKey, execution, referer string) (string, error) {
	formData := url.Values{
		"username":        {user},
		"type":            {"UsernamePassword"},
		"_eventId":        {"submit"},
		"geolocation":     {""},
		"execution":       {execution},
		"password":        {encPass},
		"croypto":         {cryptoKey},
		"captcha_code":    {""},
		"captcha_payload": {""},
	}

	req, _ := http.NewRequest("POST", loginURL, strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", referer)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("登录POST请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("登录失败，预期状态码302，实际为 %d", resp.StatusCode)
	}

	location, err := resp.Location()
	if err != nil {
		return "", fmt.Errorf("登录重定向失败，无法获取Location头: %w", err)
	}
	return location.String(), nil
}

func (s *AuthService) exchangeTicketForToken(client *http.Client, ticketURL, referer string) (string, error) {
	maxRedirects := 10
	currentURL := ticketURL
	for i := 0; i < maxRedirects; i++ {
		req, err := http.NewRequest("GET", currentURL, nil)
		if err != nil {
			return "", fmt.Errorf("创建重定向请求失败: %w", err)
		}
		req.Header.Add("Referer", referer)
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("访问重定向URL '%s' 失败: %w", currentURL, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusFound {
			location := resp.Header.Get("Location")
			if location == "" {
				return "", errors.New("服务器返回302但没有提供Location头")
			}
			parsedLocation, err := url.Parse(location)
			if err != nil {
				return "", fmt.Errorf("解析Location URL失败: %w", err)
			}

			if strings.Contains(parsedLocation.Fragment, "token=") {
				log.Println("    - 成功！在Location URL的Fragment中找到Token！")

				fragmentQuery := strings.TrimPrefix(parsedLocation.Fragment, "?")
				values, err := url.ParseQuery(fragmentQuery)
				if err != nil {
					return "", fmt.Errorf("解析URL Fragment失败: %w", err)
				}
				token := values.Get("token")
				if token != "" {
					return token, nil
				}
			}

			referer = currentURL
			currentURL = location
			continue
		}

		log.Println("    - 重定向结束，状态码:", resp.StatusCode)
		sklURL, _ := url.Parse("https://skl.hdu.edu.cn")
		cookies := client.Jar.Cookies(sklURL)
		for _, cookie := range cookies {
			if cookie.Name == "X-Auth-Token" {
				log.Println("    - 备用方案：在最终页面的Cookie中找到Token。")

				return cookie.Value, nil
			}
		}

		return "", errors.New("已完成所有重定向，但仍未在任何步骤的URL Fragment或最终Cookie中找到Token")
	}

	return "", fmt.Errorf("超过最大重定向次数 (%d)，仍未找到Token", maxRedirects)
}

func generateStateToken(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
