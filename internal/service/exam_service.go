package service

import (
	"HDU-Auto-Word-Ans-Online-Backend/internal/client"
	"HDU-Auto-Word-Ans-Online-Backend/internal/model"
	"HDU-Auto-Word-Ans-Online-Backend/internal/repository"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type ExamService struct {
	hduClient      *client.HduApiClient
	aiService      *AIService
	wordRepo       *repository.WordRepository
	answerBankRepo *repository.AnswerBankRepository
}

func NewExamService(hduClient *client.HduApiClient, aiService *AIService, wordRepo *repository.WordRepository, answerBankRepo *repository.AnswerBankRepository) *ExamService {
	return &ExamService{
		hduClient:      hduClient,
		aiService:      aiService,
		wordRepo:       wordRepo,
		answerBankRepo: answerBankRepo,
	}
}

func generateQuestionFingerprint(q model.Question) string {
	title := strings.TrimSpace(strings.TrimRight(q.Title, ". "))
	optA := strings.TrimSpace(strings.TrimRight(q.AnswerA, ". "))
	optB := strings.TrimSpace(strings.TrimRight(q.AnswerB, ". "))
	optC := strings.TrimSpace(strings.TrimRight(q.AnswerC, ". "))
	optD := strings.TrimSpace(strings.TrimRight(q.AnswerD, ". "))
	return fmt.Sprintf("%s|%s|%s|%s|%s", title, optA, optB, optC, optD)
}

func (s *ExamService) GetCurrentWeek(xAuthToken string) (int, error) {
	courseInfo, err := s.hduClient.FetchCurrentWeek(xAuthToken)
	if err != nil {
		return 0, err
	}

	if courseInfo.Week == 0 {
		return 0, fmt.Errorf("API返回的周数 'week' 为0或不存在")
	}

	return courseInfo.Week, nil
}

func isEnglish(s string) bool {
	if len(s) == 0 {
		return false
	}
	firstChar := rune(s[0])
	return (firstChar >= 'a' && firstChar <= 'z') || (firstChar >= 'A' && firstChar <= 'Z')
}

func (s *ExamService) ProcessTest(xAuthToken string, delaySeconds int, week int, examType int) (string, error) {
	startTime := time.Now()
	fmt.Println("开始处理新的测试请求...")

	paper, err := s.hduClient.GetNewPaper(xAuthToken, week, fmt.Sprintf("%d", examType))

	if err != nil {
		if errors.Is(err, client.ErrRateLimited) {
			return "", err
		}
		return "获取试卷失败", err
	}
	fmt.Printf("成功获取试卷，ID: %s\n", paper.PaperID)

	var unsolvedQuestions []model.Question
	finalAnswers := make(map[string]string)
	var mu sync.Mutex
	bankHitCount := 0
	dbHitCount := 0

	for _, q := range paper.List {
		var foundAnswer string
		fingerprint := generateQuestionFingerprint(q)
		answer, found := s.answerBankRepo.Query(fingerprint)
		if found {
			foundAnswer = answer
			bankHitCount++
		} else {
			title := strings.TrimSpace(strings.TrimRight(q.Title, ". "))
			options := map[string]string{
				"A": strings.TrimSpace(strings.TrimRight(q.AnswerA, ". ")),
				"B": strings.TrimSpace(strings.TrimRight(q.AnswerB, ". ")),
				"C": strings.TrimSpace(strings.TrimRight(q.AnswerC, ". ")),
				"D": strings.TrimSpace(strings.TrimRight(q.AnswerD, ". ")),
			}

			if isEnglish(title) {
				fullDefinition := s.wordRepo.FindDefinitionByWord(title)
				if fullDefinition != "" {
					for optionKey, optionValue := range options {
						if strings.Contains(fullDefinition, optionValue) {
							foundAnswer = optionKey
							break
						}
					}
				}
			} else {
				correctWord := s.wordRepo.FindWordByMeaning(title)
				if correctWord != "" {
					for optionKey, optionValue := range options {
						if optionValue == correctWord {
							foundAnswer = optionKey
							break
						}
					}
				}
			}

			if foundAnswer != "" {
				dbHitCount++
			}
		}

		if foundAnswer != "" {
			mu.Lock()
			finalAnswers[q.PaperDetailID] = foundAnswer
			mu.Unlock()
		} else {
			unsolvedQuestions = append(unsolvedQuestions, q)
		}
	}

	// 统计命中情况
	fmt.Printf("答案银行命中 %d 题，PDF题库命中 %d 题，有 %d 题待AI解决。\n", bankHitCount, dbHitCount, len(unsolvedQuestions))

	aiSolvedCount := 0
	if len(unsolvedQuestions) > 0 {
		fmt.Printf("正在将 %d 个问题批量提交给AI...\n", len(unsolvedQuestions))
		aiAnswers, err := s.aiService.BatchGetAnswersFromAI(unsolvedQuestions)
		if err != nil {
			fmt.Printf("AI批量处理失败: %v。正在回退到逐个问题处理模式...\n", err)
			for _, q := range unsolvedQuestions {
				fmt.Printf("... 正在单独处理问题: '%s'\n", q.Title)
				singleAnswer, singleErr := s.aiService.GetAnswerFromAI(q)
				if singleErr != nil {
					fmt.Printf("警告: 单独处理问题 '%s' (ID: %s) 失败: %v\n", q.Title, q.PaperDetailID, singleErr)
					continue
				}
				mu.Lock()
				finalAnswers[q.PaperDetailID] = singleAnswer
				mu.Unlock()
				aiSolvedCount++
			}
			fmt.Printf("逐个问题处理完成，成功获取 %d 个答案。\n", aiSolvedCount)
		} else {
			mu.Lock()
			for i, question := range unsolvedQuestions {
				if i < len(aiAnswers) {
					finalAnswers[question.PaperDetailID] = aiAnswers[i]
				}
			}
			mu.Unlock()
			aiSolvedCount = len(aiAnswers)
			fmt.Printf("AI成功返回 %d 个答案，已合并。\n", len(aiAnswers))
		}
	}

	submissionList := make([]model.AnswerInput, len(paper.List))
	for i, q := range paper.List {
		submissionList[i] = model.AnswerInput{PaperDetailID: q.PaperDetailID}
		if answer, ok := finalAnswers[q.PaperDetailID]; ok {
			finalAnswer := answer
			submissionList[i].Input = &finalAnswer
		}
	}

	submission := model.SubmissionPayload{
		PaperID: paper.PaperID,
		Type:    "0",
		List:    submissionList,
	}

	if delaySeconds > 0 {
		elapsed := time.Since(startTime)

		totalDuration := time.Duration(delaySeconds) * time.Second
		fmt.Printf("所有答案已准备就绪。将等待 %d 秒后提交...\n", delaySeconds)
		waitTime := totalDuration - elapsed + 300*time.Millisecond
		if waitTime > 0 {
			fmt.Printf("答案计算耗时 %.2f 秒。将再等待 %.2f 秒以达到总时长 %d 秒后提交...\n", elapsed.Seconds(), waitTime.Seconds(), delaySeconds)
			time.Sleep(waitTime)
		} else {
			fmt.Printf("答案计算耗时 %.2f 秒，已超过设定的 %d 秒延迟，将立即提交。\n", elapsed.Seconds(), delaySeconds)
		}
	}

	fmt.Println("正在提交试卷...")

	if err := s.hduClient.SubmitPaper(xAuthToken, &submission); err != nil {
		return "提交试卷失败", err
	}
	fmt.Println("测试请求处理成功！")
	go s.learnFromTestResult(xAuthToken, paper.PaperID)
	return fmt.Sprintf("自动化测试成功完成并提交！题库命中 %d, AI成功处理 %d。", dbHitCount, aiSolvedCount), nil
}

func (s *ExamService) learnFromTestResult(xAuthToken, paperID string) {
	log.Printf("[Learn] 学习协程已启动 (PaperID: %s)", paperID)
	learningDelay := 5 * time.Second
	log.Printf("[Learn] 将等待 %v 后开始获取答案...", learningDelay)
	time.Sleep(learningDelay)

	log.Printf("[Learn] 正在为试卷 %s 获取官方答案...", paperID)

	detail, err := s.hduClient.FetchPaperDetail(xAuthToken, paperID)
	if err != nil {
		log.Printf("!!! 致命错误 (考后学习): 获取试卷详情时出错: %v", err)
		log.Printf("[Learn] 学习协程异常退出 (PaperID: %s)", paperID)
		return
	}
	log.Printf("[Learn] 成功获取试卷详情，共 %d 道题。", len(detail.List))

	newAnswersToSave := make(map[string]string)
	for _, item := range detail.List {
		q := model.Question{
			Title:   item.Title,
			AnswerA: item.AnswerA,
			AnswerB: item.AnswerB,
			AnswerC: item.AnswerC,
			AnswerD: item.AnswerD,
		}
		fingerprint := generateQuestionFingerprint(q)
		newAnswersToSave[fingerprint] = item.Answer
	}
	log.Printf("[Learn] 已从详情中提取 %d 条答案准备存入银行。", len(newAnswersToSave))

	if err := s.answerBankRepo.Save(newAnswersToSave); err != nil {
		log.Printf("!!! 致命错误 (考后学习): 保存到答案银行时出错: %v", err)
		log.Printf("[Learn] 学习协程异常退出 (PaperID: %s)", paperID)
		return
	}
	log.Printf("[Learn] 学习协程成功完成 (PaperID: %s)", paperID)
}
