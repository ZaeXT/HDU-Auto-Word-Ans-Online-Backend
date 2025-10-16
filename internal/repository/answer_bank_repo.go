package repository

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
)

type AnswerBankRepository struct {
	filePath string
	mu       sync.RWMutex
	bank     map[string]string
}

func NewAnswerBankRepository(filePath string) (*AnswerBankRepository, error) {
	repo := &AnswerBankRepository{
		filePath: filePath,
		bank:     make(map[string]string),
	}
	if err := repo.load(); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("答案银行文件不存在，将创建一个新的。")
			if err := repo.persist(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	fmt.Printf("答案银行加载完成，当前包含 %d 条已验证答案。\n", len(repo.bank))
	log.Printf("[AnswerBank] 仓库已初始化，文件路径: '%s'", filePath)
	return repo, nil
}

func (r *AnswerBankRepository) load() error {
	log.Println("[AnswerBank] 正在尝试加锁 (读取) 并加载文件...")
	r.mu.Lock()
	defer func() {
		r.mu.Unlock()
		log.Println("[AnswerBank] 解锁 (读取) 完成。")
	}()

	byteValue, err := os.ReadFile(r.filePath)
	if err != nil {
		return err
	}

	if len(byteValue) == 0 {
		r.bank = make(map[string]string)
		log.Println("[AnswerBank] 加载完成: 文件为空，已初始化空题库。")
		return nil
	}

	err = json.Unmarshal(byteValue, &r.bank)
	if err != nil {
		log.Printf("[AnswerBank] 加载失败: 解析JSON错误: %v", err)
	} else {
		log.Printf("[AnswerBank] 加载完成: 成功解析 %d 条记录。", len(r.bank))
	}

	return err
}

func (r *AnswerBankRepository) persist() error {
	log.Println("[AnswerBank] 正在尝试加锁 (写入) 并持久化文件...")
	byteValue, err := json.MarshalIndent(r.bank, "", "  ")
	if err != nil {
		log.Printf("[AnswerBank] 持久化失败: 序列化JSON错误: %v", err)
		return err
	}

	err = os.WriteFile(r.filePath, byteValue, 0644)
	if err != nil {
		log.Printf("[AnswerBank] 持久化失败: 写入文件错误: %v", err)
	} else {
		log.Printf("[AnswerBank] 持久化成功: %d 条记录已写入 '%s'。", len(r.bank), r.filePath)
	}
	return err
}

func (r *AnswerBankRepository) Query(fingerprint string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	answer, found := r.bank[fingerprint]
	return answer, found
}

func (r *AnswerBankRepository) Save(newAnswers map[string]string) error {
	log.Println("[AnswerBank] 收到保存新答案的请求...")
	r.mu.Lock()
	defer func() {
		r.mu.Unlock()
		log.Println("[AnswerBank] 解锁 (写入) 完成。")
	}()
	log.Println("[AnswerBank] 已获取写锁。")

	addedCount := 0
	for fingerprint, answer := range newAnswers {
		if _, exists := r.bank[fingerprint]; !exists {
			r.bank[fingerprint] = answer
			addedCount++
		}
	}

	if addedCount > 0 {
		log.Printf("[AnswerBank] 发现 %d 条新答案，准备持久化...", addedCount)
		return r.persist()
	}

	log.Println("[AnswerBank] 没有需要学习的新答案。")
	return nil
}
