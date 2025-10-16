package main

import (
	"HDU-Auto-Word-Ans-Online-Backend/internal/api"
	"HDU-Auto-Word-Ans-Online-Backend/internal/auth"
	"HDU-Auto-Word-Ans-Online-Backend/internal/client"
	"HDU-Auto-Word-Ans-Online-Backend/internal/repository"
	"HDU-Auto-Word-Ans-Online-Backend/internal/router"
	"HDU-Auto-Word-Ans-Online-Backend/internal/service"
	"fmt"
	"log"
	"os"

	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %s", err)
	}
	jsonPath := viper.GetString("database.json_path")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		log.Fatalf("题库文件不存在: %s. 请先运行Python转换脚本。", jsonPath)
	}
	wordRepo, err := repository.NewWordRepository(jsonPath)
	if err != nil {
		log.Fatalf("初始化题库失败: %s", err)
	}

	answerBankPath := viper.GetString("database.answer_bank_path")
	answerBankRepo, err := repository.NewAnswerBankRepository(answerBankPath)
	if err != nil {
		log.Fatalf("初始化答案银行失败: %s", err)
	}

	hduClient := client.NewHduApiClient(viper.GetString("hdu_api.base_url"), viper.GetInt("hdu_api.timeout_seconds"))
	aiService := service.NewAIService(
		viper.GetString("deepseek_ai.base_url"),
		viper.GetString("deepseek_ai.api_key"),
		viper.GetInt("deepseek_ai.timeout_seconds"),
	)

	authService, err := auth.NewAuthService()
	if err != nil {
		log.Fatalf("初始化认证服务失败: %s", err)
	}

	examService := service.NewExamService(hduClient, aiService, wordRepo, answerBankRepo)

	examHandler := api.NewExamHandler(examService, authService)

	r := router.SetupRouter(examHandler, viper.GetStringSlice("cors.allowed_origins"))

	serverPort := viper.GetString("server.port")
	fmt.Printf("服务启动于 http://localhost%s\n", serverPort)
	if err := r.Run(serverPort); err != nil {
		log.Fatalf("服务启动失败: %s", err)
	}
}
