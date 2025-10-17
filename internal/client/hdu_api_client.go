package client

import (
	"HDU-Auto-Word-Ans-Online-Backend/internal/model"
	"HDU-Auto-Word-Ans-Online-Backend/internal/utils"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

var ErrRateLimited = errors.New("request frequency is too fast")

type HduApiClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewHduApiClient(baseURL string, timeoutSec int) *HduApiClient {
	return &HduApiClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

func logRequest(req *http.Request, description string) {
	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		log.Printf("!!! 错误：无法导出请求 '%s': %v", description, err)
		return
	}
	log.Printf("--- HTTP Request Sent: %s ---\n%s\n---------------------------------------\n", description, string(dump))
}

func setCommonHeaders(req *http.Request, xAuthToken, sklTicket string) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "https://skl.hdu.edu.cn/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36")
	req.Header.Set("X-Auth-Token", xAuthToken)
	req.Header.Set("skl-ticket", sklTicket)
}

func (C *HduApiClient) FetchCurrentWeek(xAuthToken string) (*model.CourseInfoResponse, error) {
	sklTicket, err := utils.GenerateSklTicket()
	if err != nil {
		return nil, fmt.Errorf("为获取周数生成票据失败: %w", err)
	}
	today := time.Now().Format("2006-01-02")
	url := fmt.Sprintf("%s/course?startTime=%s", C.BaseURL, today)

	req, _ := http.NewRequest("GET", url, nil)
	setCommonHeaders(req, xAuthToken, sklTicket)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	// logRequest(req, "Fetch Current Week")

	resp, err := C.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取当前周数请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取当前周数API返回错误状态: %s", resp.Status)
	}

	var courseInfo model.CourseInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&courseInfo); err != nil {
		return nil, fmt.Errorf("解析周数响应失败: %w", err)
	}

	return &courseInfo, nil
}

func (c *HduApiClient) FetchPaperDetail(xAuthToken, paperID string) (*model.PaperDetailResponse, error) {
	sklTicket, err := utils.GenerateSklTicket()
	if err != nil {
		return nil, fmt.Errorf("为获取试卷详情生成票据失败: %w", err)
	}
	url := fmt.Sprintf("%s/paper/detail?paperId=%s", c.BaseURL, paperID)

	req, _ := http.NewRequest("GET", url, nil)
	setCommonHeaders(req, xAuthToken, sklTicket)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	// logRequest(req, "Fetch Paper Detail")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			log.Printf("!!! 致命错误 (考后学习): 获取试卷详情时发生网络超时 (配置的超时时间为 %s)", c.HTTPClient.Timeout.String())
		}
		return nil, fmt.Errorf("获取试卷详情请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("!!! 错误 (考后学习): 获取试卷详情API返回非200状态。状态码: %s, 响应体: %s", resp.Status, string(bodyBytes))
		return nil, fmt.Errorf("获取试卷详情API返回错误状态: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取试卷详情响应体失败: %w", err)
	}
	log.Printf("--- Received Paper Detail Response --- (Size: %d bytes)\n", len(bodyBytes))

	var detailResponse model.PaperDetailResponse
	if err := json.Unmarshal(bodyBytes, &detailResponse); err != nil {
		log.Printf("!!! 致命错误 (考后学习): 解析试卷详情JSON失败。原始响应体如下: !!!\n%s\n!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n", string(bodyBytes))
		return nil, fmt.Errorf("解析试卷详情响应失败: %w", err)
	}

	return &detailResponse, nil
}

func (c *HduApiClient) GetNewPaper(xAuthToken string, week int, examType string) (*model.PaperResponse, error) {
	sklTicket, err := utils.GenerateSklTicket()
	if err != nil {
		return nil, fmt.Errorf("为获取试卷生成票据失败: %w", err)
	}
	startTime := time.Now().UnixMilli()
	url := fmt.Sprintf("%s/paper/new?type=%s&week=%d&startTime=%d", c.BaseURL, examType, week, startTime)

	req, _ := http.NewRequest("GET", url, nil)
	setCommonHeaders(req, xAuthToken, sklTicket)

	// logRequest(req, "Get New Paper")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送获取试卷请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusBadRequest {
			var errorResponse model.ErrorResponse
			if json.Unmarshal(bodyBytes, &errorResponse) == nil {
				if errorResponse.Code == 2 && strings.Contains(errorResponse.Msg, "请勿在短时间重试") {
					log.Println("检测到请求频率过快错误。")
					return nil, ErrRateLimited
				}
			}
		}
		return nil, fmt.Errorf("获取试卷API返回错误状态: %s", resp.Status)
	}

	var paperResponse model.PaperResponse
	if err := json.NewDecoder(resp.Body).Decode(&paperResponse); err != nil {
		return nil, fmt.Errorf("解析试卷响应失败: %w", err)
	}

	return &paperResponse, nil
}

func (c *HduApiClient) SubmitPaper(xAuthToken string, payload *model.SubmissionPayload) error {
	sklTicket, err := utils.GenerateSklTicket()
	if err != nil {
		return fmt.Errorf("为提交试卷生成票据失败: %w", err)
	}
	payloadBytes, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/paper/save", c.BaseURL)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))

	setCommonHeaders(req, xAuthToken, sklTicket)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://skl.hdu.edu.cn")
	// logRequest(req, "Submit Paper")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("提交API返回错误状态: %s", resp.Status)
	}

	return nil
}
