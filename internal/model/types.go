package model

type PaperResponse struct {
	PaperID string     `json:"paperId"`
	List    []Question `json:"list"`
}

type Question struct {
	PaperDetailID string `json:"paperDetailId"`
	Title         string `json:"title"`
	AnswerA       string `json:"answerA"`
	AnswerB       string `json:"answerB"`
	AnswerC       string `json:"answerC"`
	AnswerD       string `json:"answerD"`
}

type SubmissionPayload struct {
	PaperID string        `json:"paperId"`
	Type    string        `json:"type"`
	List    []AnswerInput `json:"list"`
}

type AnswerInput struct {
	Input         *string `json:"input,omitempty"`
	PaperDetailID string  `json:"paperDetailId"`
}

type AIChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

type CourseInfoResponse struct {
	Week int `json:"week"`
}

type PaperDetailResponse struct {
	PaperID string           `json:"paperId"`
	Mark    int              `json:"mark"`
	List    []QuestionDetail `json:"list"`
}

type QuestionDetail struct {
	PaperDetailID string  `json:"paperDetailId"`
	Title         string  `json:"title"`
	AnswerA       string  `json:"answerA"`
	AnswerB       string  `json:"answerB"`
	AnswerC       string  `json:"answerC"`
	AnswerD       string  `json:"answerD"`
	Answer        string  `json:"answer"`
	Input         *string `json:"input"`
	Right         bool    `json:"right"`
}

type ErrorResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
