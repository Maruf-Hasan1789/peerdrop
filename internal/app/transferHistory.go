package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/labstack/gommon/log"
)

type transferHistoryList struct {
	SentFileHistories []transferHistory
	historyLock       sync.Mutex
}

type transferHistory struct {
	Receiver  string `json:"receiver"`
	FileName  string `json:"fileName"`
	Status    string `json:"status"`
	TimeStamp int64  `json:"timeStamp"`
}

var transferHistories transferHistoryList

func loadOrCreateSentTransferHistory() ([]transferHistory, error) {
	log.Info("Loading sent transfer history")
	sentHistoryFile, err := getHistoryFilePath()

	if err != nil {
		log.Printf("Error loading sent transfer history: %v", err)
		return nil, err
	}

	if _, err := os.Stat(sentHistoryFile); os.IsNotExist(err) {
		log.Printf("Sent transfer history not found")
		err := os.WriteFile(sentHistoryFile, []byte("[]"), 0644)

		if err != nil {
			log.Printf("Error creating sent transfer history: %v", err)
			return nil, err
		}
	}

	sentFilesHistory, err := readSentFileHistory(sentHistoryFile)
	if err != nil {
		log.Printf("Error loading sent transfer history: %v", err)
		return nil, err
	}

	return sentFilesHistory, nil
}

func readSentFileHistory(historyFilePath string) ([]transferHistory, error) {
	data, err := os.ReadFile(historyFilePath)

	if err != nil {
		return nil, err
	}

	var sentHistory []transferHistory
	err = json.Unmarshal(data, &sentHistory)

	if err != nil {
		return nil, err
	}

	return sentHistory, nil
}

func getHistoryFilePath() (string, error) {
	userConfigDir, err := os.UserConfigDir()

	if err != nil {
		return "", fmt.Errorf("error getting user config dir: %v", err)
	}

	transferConfigDir := filepath.Join(userConfigDir, "PeerDrop", "transferHistory")

	err = os.MkdirAll(transferConfigDir, 0755)

	if err != nil {
		return "", fmt.Errorf("error creating transfer history directory: %v", err)
	}

	historyFile := filepath.Join(transferConfigDir, "sent_transfer_history.json")

	return historyFile, nil
}

func addNewSentFileHistory(receiver string, fileName string, status string) error {
	sentFileHistories, err := loadOrCreateSentTransferHistory()

	if err != nil {
		return err
	}

	transferHistories.historyLock.Lock()
	defer transferHistories.historyLock.Unlock()

	entry := transferHistory{
		Receiver: receiver,
		FileName: fileName,
		Status:   status,
	}

	transferHistories.SentFileHistories = append(transferHistories.SentFileHistories, sentFileHistories...)
	transferHistories.SentFileHistories = append(transferHistories.SentFileHistories, entry)

	const maxEntries = 500

	if len(transferHistories.SentFileHistories) > maxEntries {
		transferHistories.SentFileHistories = transferHistories.SentFileHistories[:maxEntries]
	}

	err = saveSentFileHistoriesInDisk(transferHistories.SentFileHistories)

	if err != nil {
		return err
	}

	return err
}

func saveSentFileHistoriesInDisk(sentFilesList []transferHistory) error {

	data, err := json.MarshalIndent(sentFilesList, "", "  ")

	if err != nil {
		return err
	}

	tmpFile, err := getHistoryFilePath()
	if err != nil {
		return err
	}

	temporaryFile := tmpFile + ".tmp"

	if err := os.WriteFile(temporaryFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(temporaryFile, tmpFile)
}
