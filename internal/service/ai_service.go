package service

import (
	"HDU-Auto-Word-Ans-Online-Backend/internal/model"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type AIService struct {
	BaseURL    string
	APIKey     string
	Model      string
	HttpClient *http.Client
}

func NewAIService(baseURL, apiKey, model string, timeoutSec int) *AIService {
	return &AIService{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		HttpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

func (s *AIService) GetAnswerFromAI(q model.Question) (string, error) {
	prompt := fmt.Sprintf(`
你要做的是词义匹配，找到和问题最贴切的选项。
最终只回答一个被'-'包起来的大写字母作为答案, 例如"-B-"。
不要包含任何其他解释或文字。

问题: %s
A. %s
B. %s
C. %s
D. %s`,
		q.Title, q.AnswerA, q.AnswerB, q.AnswerC, q.AnswerD)

	isEnglishQuestion := (q.Title[0] >= 'a' && q.Title[0] <= 'z') || (q.Title[0] >= 'A' && q.Title[0] <= 'Z')
	var systemPrompt string
	if isEnglishQuestion {
		systemPrompt = "你要做的是词义匹配，找到和英文单词最贴切的中文解释"
	} else {
		systemPrompt = "你要做的是词义匹配，找到和中文意思最贴切的英语单词"
	}

	payload := model.AIChatRequest{
		Model: s.Model,
		Messages: []model.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	log.Printf("--- Sending SINGLE request to AI ---\n")

	req, _ := http.NewRequest("POST", s.BaseURL+"/chat/completions", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	resp, err := s.HttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var aiResponse model.AIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResponse); err != nil {
		return "", err
	}

	if len(aiResponse.Choices) > 0 {
		content := aiResponse.Choices[0].Message.Content
		log.Printf("--- Received SINGLE response from AI ---\n%s\n--------------------------------------\n", content)
		re := regexp.MustCompile(`-([A-D])-`)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("AI未能按预期格式返回答案")
}

func (s *AIService) BatchGetAnswersFromAI(questions []model.Question) ([]string, error) {
	var promptBuilder strings.Builder
	promptBuilder.WriteString("你需要一次性解决以下所有词义匹配问题。\n")
	promptBuilder.WriteString("请严格按照问题的顺序，在独立的一行中只回答一个大写字母选项 (A, B, C, 或 D)。\n")
	promptBuilder.WriteString(fmt.Sprintf("总共有 %d 个问题，所以你的回答也应该恰好是 %d 行，每行只有一个字母。\n\n", len(questions), len(questions)))

	for i, q := range questions {
		// 不再发送问题ID，只按顺序提问
		promptBuilder.WriteString(fmt.Sprintf("--- 问题 %d ---\n", i+1))
		promptBuilder.WriteString(fmt.Sprintf("题目: %s\n", q.Title))
		promptBuilder.WriteString(fmt.Sprintf("A. %s\n", q.AnswerA))
		promptBuilder.WriteString(fmt.Sprintf("B. %s\n", q.AnswerB))
		promptBuilder.WriteString(fmt.Sprintf("C. %s\n", q.AnswerC))
		promptBuilder.WriteString(fmt.Sprintf("D. %s\n\n", q.AnswerD))
	}

	systemPrompt := "你是一个高效的英语词义匹配助手，你需要根据指令批量处理问题并严格按顺序、按指定格式返回结果。"

	payload := model.AIChatRequest{
		Model: s.Model,
		Messages: []model.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: promptBuilder.String()},
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	log.Printf("--- Sending SINGLE request to AI ---\n%s\n--------------------------------------\n", string(payloadBytes))
	req, _ := http.NewRequest("POST", s.BaseURL+"/chat/completions", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	resp, err := s.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取AI响应体失败: %w", err)
	}
	var aiResponse model.AIChatResponse
	if err := json.Unmarshal(bodyBytes, &aiResponse); err != nil {
		// 如果JSON解析失败，打印原始的响应体，这有助于发现问题
		log.Printf("!!! 错误: 解析AI批量响应JSON失败。原始响应体如下: !!!\n%s\n!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n", string(bodyBytes))
		return nil, fmt.Errorf("解析AI批量响应JSON失败: %w", err)
	}

	var answers []string
	if len(aiResponse.Choices) > 0 {
		content := aiResponse.Choices[0].Message.Content
		log.Printf("--- Received SINGLE response from AI ---\n%s\n--------------------------------------\n", content)
		re := regexp.MustCompile(`(?m)^([A-D])$`)

		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) == 2 {
				answers = append(answers, match[1])
			}
		}
	} else {
		log.Printf("--- Received BATCH response from AI (but choices array is empty) ---\nRaw Body: %s\n---------------------------------------\n", string(bodyBytes))
	}

	if len(answers) != len(questions) {
		fmt.Printf("警告: AI返回的答案数量 (%d) 与问题数量 (%d) 不匹配。\n", len(answers), len(questions))
		return nil, fmt.Errorf("AI返回的答案数量 (%d) 与问题数量 (%d) 不匹配", len(answers), len(questions))
	}

	return answers, nil
}
