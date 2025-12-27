package processreqs

import (
	"encoding/json"
	"fmt"
	"strings"
	"regexp"
	"sync"
	"time"

	sr "cavalier/pkg/speechrequest"
	"cavalier/pkg/vars"
	"cavalier/pkg/vtt"

	pb "github.com/digital-dream-labs/api/go/chipperpb"
	"github.com/pkg/errors"
	"github.com/soundhound/houndify-sdk-go"
)

var HKGclient houndify.Client
var HoundEnable bool = true


var houndifyCache = make(map[string]cacheEntry)
var cacheMutex sync.RWMutex

type cacheEntry struct {
	response  string
	timestamp time.Time
}

func ParseSpokenResponse(serverResponseJSON string) (string, error) {
	result := make(map[string]interface{})
	err := json.Unmarshal([]byte(serverResponseJSON), &result)
	if err != nil {
		fmt.Println(err.Error())
		return "", errors.New("failed to decode json")
	}
	if !strings.EqualFold(result["Status"].(string), "OK") {
		return "", errors.New(result["ErrorMessage"].(string))
	}
	if result["NumToReturn"].(float64) < 1 {
		return "", errors.New("no results to return")
	}
	return result["AllResults"].([]interface{})[0].(map[string]interface{})["SpokenResponseLong"].(string), nil
}

func InitKnowledge() {
	if vars.APIConfig.Knowledge.Enable && vars.APIConfig.Knowledge.Provider == "houndify" {
		if vars.APIConfig.Knowledge.ID == "" || vars.APIConfig.Knowledge.Key == "" {
			vars.APIConfig.Knowledge.Enable = false
			fmt.Println("Houndify Client Key or ID was empty, not initializing kg client")
		} else {
			HKGclient = houndify.Client{
				ClientID:  vars.APIConfig.Knowledge.ID,
				ClientKey: vars.APIConfig.Knowledge.Key,
			}
			HKGclient.EnableConversationState()
			fmt.Println("Initialized Houndify client")
		}
	}
}

var NoResult string = "NoResultCommand"
var NoResultSpoken string

func houndifyKG(req sr.SpeechRequest) string {
	var apiResponse string
	if vars.APIConfig.Knowledge.Enable && vars.APIConfig.Knowledge.Provider == "houndify" {
		fmt.Println("Sending request to Houndify...")
		serverResponse := StreamAudioToHoundify(req, HKGclient)
		apiResponse, _ = ParseSpokenResponse(serverResponse)
		fmt.Println("Houndify response: " + apiResponse)
	} else {
		apiResponse = "Houndify is not enabled."
		fmt.Println("Houndify is not enabled.")
	}
	return apiResponse
}

// Takes a SpeechRequest, figures out knowledgegraph provider, makes request, returns API response
func KgRequest(req *vtt.KnowledgeGraphRequest, speechReq sr.SpeechRequest) string {
	if vars.APIConfig.Knowledge.Enable {
		if vars.APIConfig.Knowledge.Provider == "houndify" {
			return houndifyKG(speechReq)
		}
	}
	return "Knowledge graph is not enabled. This can be enabled in the web interface."
}

func (s *Server) ProcessKnowledgeGraph(req *vtt.KnowledgeGraphRequest) (*vtt.KnowledgeGraphResponse, error) {
	InitKnowledge()
	speechReq := sr.ReqToSpeechRequest(req)
	if vars.APIConfig.Knowledge.Enable && vars.APIConfig.Knowledge.Provider == "houndify" {
		apiResponse := KgRequest(req, speechReq)
		
		// Check if response is empty or contains error
		var spokenResponse string
		if apiResponse == "" || strings.TrimSpace(apiResponse) == "" {
			fmt.Println("Houndify knowledge graph returned error/empty, I'm prolly out of credits again")
			spokenResponse = "Sorry for the inconvenience, I've most likely ran out of houndify credits for today and can't process this knowledge graph request. Please try again later."
		} else {
			spokenResponse = apiResponse
		}
		
		kg := pb.KnowledgeGraphResponse{
			Session:     req.Session,
			DeviceId:    req.Device,
			CommandType: NoResult,
			SpokenText:  spokenResponse,
		}
		fmt.Println("(KG) Bot " + speechReq.Device + " request served.")
		if err := req.Stream.Send(&kg); err != nil {
			return nil, err
		}
	}
	return nil, nil

}

func cleanHoundifyResponse(response string) string {
	// This should remove the "Redirected from" text
	re := regexp.MustCompile(`^Redirected from [^.]+\.\s*`)
	cleaned := re.ReplaceAllString(response, "")
	return cleaned
}

func houndifyTextRequest(queryText string, device string, session string) string {
	if !vars.APIConfig.Knowledge.Enable || vars.APIConfig.Knowledge.Provider != "houndify" {
		return "Houndify is not enabled."
	}
	
	fmt.Println("Sending text request to Houndify...")
	
	req := houndify.TextRequest{
		Query:     queryText,
		UserID:    device,
		RequestID: session,
	}
	
	serverResponse, err := HKGclient.TextSearch(req)
	if err != nil {
		fmt.Println("Error sending text request to Houndify:", err)
		return ""
	}
	
	apiResponse, err := ParseSpokenResponse(serverResponse)
	if err != nil {
		fmt.Println("Error parsing Houndify response:", err)
		fmt.Println("Raw response:", serverResponse)
		return ""
	}
	
	apiResponse = cleanHoundifyResponse(apiResponse)
	
	fmt.Println("Houndify response:", apiResponse)
	return apiResponse
}

// getCachedOrFetch checks cache first, then fetches from Houndify if needed
func getCachedOrFetch(query string, device string, session string) string {
	cacheKey := strings.ToLower(strings.TrimSpace(query))
	
	cacheMutex.RLock()
	if entry, exists := houndifyCache[cacheKey]; exists {
		if time.Since(entry.timestamp) < 10*time.Hour {
			cacheMutex.RUnlock()
			fmt.Printf("Cache HIT for query: %s (age: %v)\n", query, time.Since(entry.timestamp))
			return entry.response
		}
	}
	cacheMutex.RUnlock()
	
	fmt.Println("Cache MISS for query:", query)
	
	response := houndifyTextRequest(query, device, session)

	if response != "" && !strings.Contains(response, "not enabled") {
		cacheMutex.Lock()
		houndifyCache[cacheKey] = cacheEntry{
			response:  response,
			timestamp: time.Now(),
		}
		if len(houndifyCache) > 100 {
			var oldestKey string
			oldestTime := time.Now()
			for k, v := range houndifyCache {
				if v.timestamp.Before(oldestTime) {
					oldestTime = v.timestamp
					oldestKey = k
				}
			}
			delete(houndifyCache, oldestKey)
		}
		cacheMutex.Unlock()
	}
	
	return response
}