package repository

import (
	"encoding/json"
	"fmt"
	"os"
)

type WordRepository struct {
	WordToDefinition map[string]string `json:"wordToDefinition"`
	MeaningToWord    map[string]string `json:"meaningToWord"`
}

func NewWordRepository(jsonPath string) (*WordRepository, error) {
	fmt.Println("正在从JSON文件加载题库...")
	byteValue, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取JSON文件 '%s': %w", jsonPath, err)
	}
	var repo WordRepository
	if err := json.Unmarshal(byteValue, &repo); err != nil {
		return nil, fmt.Errorf("解析JSON数据失败: %w", err)
	}

	fmt.Printf("题库加载完成，共加载 %d 个英文单词和 %d 个中文释义映射。\n", len(repo.WordToDefinition), len(repo.MeaningToWord))
	return &repo, nil
}

func (r *WordRepository) FindDefinitionByWord(word string) string {
	return r.WordToDefinition[word]
}

func (r *WordRepository) FindWordByMeaning(meaning string) string {
	return r.MeaningToWord[meaning]
}
