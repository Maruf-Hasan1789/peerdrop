package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/labstack/gommon/log"
)

type Settings struct {
	UserName     string `json:"user_name"`
	DownloadPath string `json:"download_path"`
}

func LoadOrCreateSettings() (*Settings, error) {
	log.Info("Loading settings")
	settingsFile, err := getSettingsFilePath()

	if err != nil {
		log.Printf("Error getting settings file path: %v", err)
		return nil, err
	}

	var settings *Settings

	if _, err = os.Stat(*settingsFile); os.IsNotExist(err) {
		settings = &Settings{
			UserName:     "User",
			DownloadPath: filepath.Join(os.Getenv("HOME"), "Downloads"),
		}

		data, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			log.Printf("Error marshalling settings: %v", err)
			return nil, fmt.Errorf("Error while marshalling settings %v\n", err)
		}

		err = os.WriteFile(*settingsFile, data, os.ModePerm)

		if err != nil {
			log.Printf("Error creating settings file %v: %v", *settingsFile, err)
			return nil, fmt.Errorf("Error while writing settings %v\n", err)
		}

		return settings, nil
	}

	settings, err = readSettingsFile(*settingsFile)

	if err != nil {
		log.Printf("Error reading settings file %v: %v", *settingsFile, err)
		return nil, err
	}

	return settings, nil
}

func getSettingsFilePath() (*string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("Error while getting user Config %v\n", err)
	}

	appConfigDir := filepath.Join(userConfigDir, "PeerDrop")

	err = os.MkdirAll(appConfigDir, os.ModePerm)

	if err != nil {
		return nil, fmt.Errorf("Error while creating user Config %v\n", err)
	}

	settingsFile := filepath.Join(appConfigDir, "settings.json	")

	return &settingsFile, nil
}

func readSettingsFile(settingsFile string) (*Settings, error) {
	var settings Settings
	log.Infof("Reading settings file %v", settingsFile)
	data, err := os.ReadFile(settingsFile)

	if err != nil {
		log.Printf("Error reading settings file %v: %v", settingsFile, err)
		return nil, fmt.Errorf("Error while reading settings %v\n", err)
	}

	err = json.Unmarshal(data, &settings)

	if err != nil {
		log.Printf("Error unmarshalling settings file %v: %v", settingsFile, err)
		return nil, fmt.Errorf("Error while unmarshalling settings %v\n", err)
	}

	fmt.Printf("Settings file contents: %v\n", string(data))
	return &settings, nil
}
