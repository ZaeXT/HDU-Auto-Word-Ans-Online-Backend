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
	"strings"

	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("HDU_APP")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("警告：未找到 config.yaml 文件，将完全依赖环境变量进行配置。")
		} else {
			log.Fatalf("读取配置文件失败: %s", err)
		}
	}
	jsonPath := viper.GetString("database.json_path")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		log.Fatalf("基础题库文件不存在: %s. 请确保它被正确打包到镜像或位于工作目录。", jsonPath)
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
		viper.GetString("ai_service.base_url"),
		viper.GetString("ai_service.api_key"),
		viper.GetString("ai_service.model"),
		viper.GetInt("ai_service.timeout_seconds"),
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
