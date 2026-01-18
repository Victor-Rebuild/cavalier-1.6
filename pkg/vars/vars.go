package vars

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
)

var (
	SessionCertEnv = "SESSION_CERT_STORAGE"
	HoundKeyEnv    = "HOUND_KEY"
	HoundIDEnv     = "HOUND_ID"
	WeatherKeyEnv  = "WEATHER_KEY"
	KeyEnv         = "KEY"
	CertEnv        = "CERT"
)

var CertPath string
var KeyPath string

var SessionCertsStorage = "./session-certs"

var IDLength = 23

var APIConfig apiConfig

type apiConfig struct {
	Weather struct {
		Enable   bool   `json:"enable"`
		Provider string `json:"provider"`
		Key      string `json:"key"`
		Unit     string `json:"unit"`
	} `json:"weather"`
	Knowledge struct {
		Enable                 bool   `json:"enable"`
		Provider               string `json:"provider"`
		Key                    string `json:"key"`
		ID                     string `json:"id"`
		Model                  string `json:"model"`
		IntentGraph            bool   `json:"intentgraph"`
		RobotName              string `json:"robotName"`
		OpenAIPrompt           string `json:"openai_prompt"`
		OpenAIVoice            string `json:"openai_voice"`
		OpenAIVoiceWithEnglish bool   `json:"openai_voice_with_english"`
		SaveChat               bool   `json:"save_chat"`
		CommandsEnable         bool   `json:"commands_enable"`
		Endpoint               string `json:"endpoint"`
	} `json:"knowledge"`
	STT struct {
		Service  string `json:"provider"`
		Language string `json:"language"`
	} `json:"STT"`
	Server struct {
		// false for ip, true for escape pod
		EPConfig bool   `json:"epconfig"`
		Port     string `json:"port"`
	} `json:"server"`
	HasReadFromEnv   bool `json:"hasreadfromenv"`
	PastInitialSetup bool `json:"pastinitialsetup"`
}

var SttInitFunc func() error

var IntentList []JsonIntent

type JsonIntent struct {
	Name              string   `json:"name"`
	Keyphrases        []string `json:"keyphrases"`
	RequireExactMatch bool     `json:"requiresexact"`
}

func LoadIntents() ([]JsonIntent, error) {
	var path string
	path = "./"

	// var matches [][]string
	// var intents []string

	if APIConfig.STT.Service == "whisper.cpp" || APIConfig.STT.Service == "whisper" {
		return LoadAllLanguageIntents(path)
	}

	jsonFile, err := os.ReadFile(path + "intent-data/" + APIConfig.STT.Language + ".json")
	var jsonIntents []JsonIntent
	if err == nil {
		err = json.Unmarshal(jsonFile, &jsonIntents)
		if err != nil {
			fmt.Println("Failed to load intents: " + err.Error())
		}

		// for _, element := range jsonIntents {
		// 	//fmt.Println("Loading intent " + strconv.Itoa(index) + " --> " + element.Name + "( " + strconv.Itoa(len(element.Keyphrases)) + " keyphrases )")
		// 	intents = append(intents, element.Name)
		// 	matches = append(matches, element.Keyphrases)
		// }
		// fmt.Println("Loaded " + strconv.Itoa(len(jsonIntents)) + " intents and " + strconv.Itoa(len(matches)) + " matches (language: " + APIConfig.STT.Language + ")")
	}
	return jsonIntents, err
}

func LoadAllLanguageIntents(basePath string) ([]JsonIntent, error) {
	intentPath := basePath + "intent-data/"
	files, err := os.ReadDir(intentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read intent-data directory: %w", err)
	}

	var allIntents []JsonIntent
	intentMap := make(map[string]JsonIntent)

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			jsonFile, err := os.ReadFile(intentPath + file.Name())
			if err != nil {
				fmt.Printf("Warning: failed to read %s: %v\n", file.Name(), err)
				continue
			}

			var jsonIntents []JsonIntent
			err = json.Unmarshal(jsonFile, &jsonIntents)
			if err != nil {
				fmt.Printf("Warning: failed to unmarshal %s: %v\n", file.Name(), err)
				continue
			}

			for _, intent := range jsonIntents {
				if existing, exists := intentMap[intent.Name]; exists {
					existing.Keyphrases = append(existing.Keyphrases, intent.Keyphrases...)
					intentMap[intent.Name] = existing
				} else {
					intentMap[intent.Name] = intent
				}
			}
		}
	}

	for _, intent := range intentMap {
		allIntents = append(allIntents, intent)
	}

	fmt.Printf("Loaded %d intents from all languages\n", len(allIntents))
	return allIntents, nil
}

func GenerateID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
	result := make([]byte, IDLength)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return ""
		}
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

func Init() {
	KeyPath = os.Getenv("KEY")
	CertPath = os.Getenv("CERT")
	os.MkdirAll(SessionCertsStorage, 0777)
	APIConfig.STT.Language = "en-US"
	APIConfig.STT.Service = "vosk"
	APIConfig.Knowledge.Enable = true
	APIConfig.Knowledge.Provider = "houndify"
	APIConfig.Knowledge.Key = os.Getenv(HoundKeyEnv)
	APIConfig.Knowledge.ID = os.Getenv(HoundIDEnv)
	APIConfig.Weather.Enable = true
	APIConfig.Weather.Key = os.Getenv(WeatherKeyEnv)
	APIConfig.Weather.Provider = "weatherapi.com"
}
