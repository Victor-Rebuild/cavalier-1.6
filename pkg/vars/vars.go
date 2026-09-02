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
	Blacklist struct {
		Enable        bool     `json:"enable"`
		EnableStealth bool     `json:"enableStealth"`
		ESNs          []string `json:"esns"`
	} `json:"blacklist"`
	KGDisable struct {
		Enable bool     `json:"enable"`
		ESNs   []string `json:"esns"`
	} `json:"KGDisable"`
	IGDisable struct {
		Enable bool     `json:"enable"`
		ESNs   []string `json:"esns"`
	} `json:"IGDisable"`
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

func IsESNBlacklisted(esn string) bool {
	if !APIConfig.Blacklist.Enable {
		return false
	}

	esn = strings.ToLower(strings.TrimSpace(esn))
	esn = strings.TrimPrefix(esn, "vic:")

	for _, blacklistedESN := range APIConfig.Blacklist.ESNs {
		blacklistedESN = strings.ToLower(strings.TrimSpace(blacklistedESN))
		blacklistedESN = strings.TrimPrefix(blacklistedESN, "vic:")

		if esn == blacklistedESN {
			return true
		}
	}

	return false
}

func UseStealthBlacklist() bool {
	if !APIConfig.Blacklist.EnableStealth {
		return false
	}

	return true
}

func IsESNIGDisabled(esn string) bool {
	if !APIConfig.IGDisable.Enable {
		return false
	}

	esn = strings.ToLower(strings.TrimSpace(esn))
	esn = strings.TrimPrefix(esn, "vic:")

	for _, igdisabledESN := range APIConfig.KGDisable.ESNs {
		igdisabledESN = strings.ToLower(strings.TrimSpace(igdisabledESN))
		igdisabledESN = strings.TrimPrefix(igdisabledESN, "vic:")

		if esn == igdisabledESN {
			return true
		}
	}

	return false
}

func IsESNKGDisabled(esn string) bool {
	if !APIConfig.KGDisable.Enable {
		return false
	}

	esn = strings.ToLower(strings.TrimSpace(esn))
	esn = strings.TrimPrefix(esn, "vic:")

	for _, kgdisabledESN := range APIConfig.KGDisable.ESNs {
		kgdisabledESN = strings.ToLower(strings.TrimSpace(kgdisabledESN))
		kgdisabledESN = strings.TrimPrefix(kgdisabledESN, "vic:")

		if esn == kgdisabledESN {
			return true
		}
	}

	return false
}

func LoadIntents() ([]JsonIntent, error) {
	var path string
	path = "./"
	jsonFile, err := os.ReadFile(path + "intent-data/" + APIConfig.STT.Language + ".json")

	// var matches [][]string
	// var intents []string
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

	LoadConfigBlacklist()
	LoadConfigIGDisable()
	LoadConfigKGDisable()

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

func LoadConfigBlacklist() {
	configPath := "./blacklist.json"
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Println("Failed to read blacklist file:", err)
			return
		}

		err = json.Unmarshal(data, &APIConfig)
		if err != nil {
			fmt.Println("Failed to parse blacklist file:", err)
			return
		}

		fmt.Println("Loaded blacklist from blacklist.json")
		if APIConfig.Blacklist.Enable {
			fmt.Printf("Blacklist enabled with %d ESNs\n", len(APIConfig.Blacklist.ESNs))
		}
	}
}

func LoadConfigIGDisable() {
	configPathIG := "./igdisable.json"
	if _, err := os.Stat(configPathIG); err == nil {
		data, err := os.ReadFile(configPathIG)
		if err != nil {
			fmt.Println("Failed to read intent graph disable file:", err)
			return
		}

		err = json.Unmarshal(data, &APIConfig)
		if err != nil {
			fmt.Println("Failed to parse intent graph disable file:", err)
			return
		}

		fmt.Println("Loaded intent graph disabled bots from igdisable.json")
		if APIConfig.IGDisable.Enable {
			fmt.Printf("Intent graph disable enabled with %d ESNs\n", len(APIConfig.IGDisable.ESNs))
		}
	}
}

func LoadConfigKGDisable() {
	configPathIG := "./kgdisable.json"
	if _, err := os.Stat(configPathIG); err == nil {
		data, err := os.ReadFile(configPathIG)
		if err != nil {
			fmt.Println("Failed to read knowledge graph disable file:", err)
			return
		}

		err = json.Unmarshal(data, &APIConfig)
		if err != nil {
			fmt.Println("Failed to parse knowledge graph disable file:", err)
			return
		}

		fmt.Println("Loaded knowledge graph disabled bots from igdisable.json")
		if APIConfig.KGDisable.Enable {
			fmt.Printf("knowledge graph disable enabled with %d ESNs\n", len(APIConfig.KGDisable.ESNs))
		}
	}
}
