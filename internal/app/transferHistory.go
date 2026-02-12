package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/labstack/gommon/log"
)

type transferHistoryList struct {
	transferFileHistories []transferHistory
	historyLock           sync.Mutex
}

type transferHistory struct {
	Peer         string `json:"peer"`
	FileName     string `json:"file_name"`
	TransferType string `json:"transfer_type"` //SENT or RECEIVED
	Status       string `json:"status"`        //COMPLETED or FAILED
	TimeStamp    int64  `json:"time_stamp"`
}

var transferHistories transferHistoryList

func loadOrCreateTransferHistory() ([]transferHistory, error) {
	log.Info("Loading transfer history")
	historyFile, err := getHistoryFilePath()

	if err != nil {
		log.Printf("Error loading sent transfer history: %v", err)
		return nil, err
	}

	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		log.Printf("Sent transfer history not found")
		err := os.WriteFile(historyFile, []byte("[]"), 0644)

		if err != nil {
			log.Printf("Error creating sent transfer history: %v", err)
			return nil, err
		}
	}

	filesHistory, err := readFileHistory(historyFile)
	if err != nil {
		log.Printf("Error loading sent transfer history: %v", err)
		return nil, err
	}

	return filesHistory, nil
}

func readFileHistory(historyFilePath string) ([]transferHistory, error) {
	data, err := os.ReadFile(historyFilePath)

	if err != nil {
		return nil, err
	}

	var histories []transferHistory
	err = json.Unmarshal(data, &histories)

	if err != nil {
		return nil, err
	}

	return histories, nil
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

	historyFile := filepath.Join(transferConfigDir, "transfer_history.json")

	return historyFile, nil
}

func addNewTransferFileHistory(peer string, fileName string, transferType string, status string, transferId string) error {
	fmt.Printf("Transfer ID for future update %v\n", transferId)
	fileHistories, err := loadOrCreateTransferHistory()

	if err != nil {
		return err
	}

	transferHistories.historyLock.Lock()
	defer transferHistories.historyLock.Unlock()

	entry := transferHistory{
		Peer:         peer,
		FileName:     fileName,
		TransferType: transferType,
		Status:       status,
		TimeStamp:    time.Now().UnixMilli(),
	}

	transferHistories.transferFileHistories = append(transferHistories.transferFileHistories, fileHistories...)
	transferHistories.transferFileHistories = append(transferHistories.transferFileHistories, entry)

	const maxEntries = 500

	if len(transferHistories.transferFileHistories) > maxEntries {
		transferHistories.transferFileHistories = transferHistories.transferFileHistories[:maxEntries]
	}

	err = saveTransferFileHistoriesInDisk(transferHistories.transferFileHistories)

	if err != nil {
		return err
	}

	return err
}

func saveTransferFileHistoriesInDisk(sentFilesList []transferHistory) error {

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

func clearTransferHistories() error {
	historyFilePath, err := getHistoryFilePath()
	if err != nil {
		log.Printf("Error loading sent transfer history: %v", err)
	}

	file, err := os.OpenFile(historyFilePath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to clear history: %v", err)
	}

	defer file.Close()

	_, err = file.Write([]byte("[]"))
	if err != nil {
		return fmt.Errorf("failed to write empty array: %v", err)
	}

	return nil
}
