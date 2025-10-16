package api

import (
	"HDU-Auto-Word-Ans-Online-Backend/internal/auth"
	"HDU-Auto-Word-Ans-Online-Backend/internal/client"
	"HDU-Auto-Word-Ans-Online-Backend/internal/service"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type StartTestRequest struct {
	Week               int `json:"week"`
	ExamType           int `json:"exam_type"` // 0:自测 1:考试
	SubmitDelaySeconds int `json:"submit_delay_seconds"`
}

type LoginAndStartRequest struct {
	Username           string `json:"username" binding:"required"`
	Password           string `json:"password" binding:"required"`
	Week               int    `json:"week"`
	ExamType           int    `json:"exam_type"` // 0:自测 1:考试
	SubmitDelaySeconds int    `json:"submit_delay_seconds"`
}

type ExamHandler struct {
	examService *service.ExamService
	authService *auth.AuthService
}

func NewExamHandler(examService *service.ExamService, authService *auth.AuthService) *ExamHandler {
	return &ExamHandler{examService: examService, authService: authService}
}

func (h *ExamHandler) handleProcessTestError(c *gin.Context, err error, contextMsg string) {
	if errors.Is(err, client.ErrRateLimited) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "请求频率过快，请稍后再试"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":   contextMsg,
		"details": err.Error(),
	})
}

func (h *ExamHandler) StartTestHandler(c *gin.Context) {
	var req StartTestRequest
	XAuthToken := c.GetHeader("X-Auth-Token")

	if XAuthToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Auth-Token header is required"})
		return
	}

	_ = c.ShouldBindJSON(&req)
	week := req.Week
	if week == 0 {
		fmt.Println("用户未提供周数，正在自动获取当前周数...")
		fetchedWeek, err := h.examService.GetCurrentWeek(XAuthToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "自动获取当前周数失败",
				"details": err.Error(),
			})
			return
		}
		week = fetchedWeek
		fmt.Printf("自动获取成功，当前周数: %d\n", week)
	}

	result, err := h.examService.ProcessTest(XAuthToken, req.SubmitDelaySeconds, week, req.ExamType)

	if err != nil {
		h.handleProcessTestError(c, err, "处理测试失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": result})
}

func (h *ExamHandler) LoginAndStartTestHandler(c *gin.Context) {
	var req LoginAndStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	xAuthToken, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "SSO 登录失败",
			"details": err.Error(),
		})
		return
	}

	week := req.Week
	if week == 0 {
		fmt.Println("用户未提供周数，正在自动获取当前周数...")
		fetchedWeek, err := h.examService.GetCurrentWeek(xAuthToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "自动获取当前周数失败 (登录成功后)",
				"details": err.Error(),
			})
			return
		}
		week = fetchedWeek
		fmt.Printf("自动获取成功，当前周数: %d\n", week)
	}

	result, err := h.examService.ProcessTest(xAuthToken, req.SubmitDelaySeconds, week, req.ExamType)
	if err != nil {
		h.handleProcessTestError(c, err, "处理测试失败 (登录成功后)")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": result, "x_auth_token": xAuthToken})
}
