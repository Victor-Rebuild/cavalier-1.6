package processreqs

import (
	"fmt"
	"strings"
	"time"

	"cavalier/pkg/vtt"

	sr "cavalier/pkg/speechrequest"
	ttr "cavalier/pkg/ttr"
	"cavalier/pkg/vars"
)

var cantProcessIntent string = "Sorry for the inconvenience, I've most likely ran out of houndify credits for today and can't process this intent graph request. Please try again later."

func (s *Server) ProcessIntentGraph(req *vtt.IntentGraphRequest) (*vtt.IntentGraphResponse, error) {
	requestStartTime := time.Now()
	
	var successMatched bool
	speechReq := sr.ReqToSpeechRequest(req)
	var transcribedText string
	var err error
	
	if !isSti {
		sttStartTime := time.Now()
		transcribedText, err = sttHandler(speechReq)
		sttDuration := time.Since(sttStartTime)
		fmt.Printf("Bot %s - STT took: %v\n", req.Device, sttDuration)
		
		if err != nil {
			ttr.IntentPass(req, "intent_system_noaudio", "voice processing error: "+err.Error(), map[string]string{"error": err.Error()}, true)
			return nil, nil
		}
		if strings.TrimSpace(transcribedText) == "" {
			ttr.IntentPass(req, "intent_system_noaudio", "", map[string]string{}, false)
			return nil, nil
		}
		
		intentStartTime := time.Now()
		successMatched = ttr.ProcessTextAll(req, transcribedText, vars.IntentList, speechReq.IsOpus)
		intentDuration := time.Since(intentStartTime)
		fmt.Printf("Bot %s - Intent matching took: %v\n", req.Device, intentDuration)
		
	} else {
		intent, slots, err := stiHandler(speechReq)
		if err != nil {
			if err.Error() == "inference not understood" {
				fmt.Println("Bot " + speechReq.Device + " No intent was matched")
				ttr.IntentPass(req, "intent_system_unmatched", "voice processing error", map[string]string{"error": err.Error()}, true)
				return nil, nil
			}
			fmt.Println(err)
			ttr.IntentPass(req, "intent_system_noaudio", "voice processing error", map[string]string{"error": err.Error()}, true)
			return nil, nil
		}
		ttr.ParamCheckerSlotsEnUS(req, intent, slots, speechReq.IsOpus, speechReq.Device)
		return nil, nil
	}
	if !successMatched {
		// If knowledge graph is enabled, send to Houndify
		if vars.APIConfig.Knowledge.Enable {
			if len([]rune(transcribedText)) >= 8 {
				fmt.Println("No intent matched, forwarding to Houndify for device " + req.Device + "...")
				InitKnowledge() // Errors without this for whatever reason even though I think it should be inited already
				
				houndifyStartTime := time.Now()
				apiResponse := houndifyTextRequest(transcribedText, req.Device, req.Session)
				houndifyDuration := time.Since(houndifyStartTime)
				fmt.Printf("Bot %s - Houndify request took: %v\n", req.Device, houndifyDuration)
				
				if apiResponse != "" && !strings.Contains(apiResponse, "not enabled") && !strings.Contains(apiResponse, "Knowledge graph is not enabled") && !strings.Contains(apiResponse, "Didn't get that!") {
					if apiResponse == "" {
						fmt.Println("Houndify intent graph returned error/empty, I'm prolly out of credits again, send the message")
						ttr.KnowledgeGraphResponseIG(req, cantProcessIntent, transcribedText)
						totalDuration := time.Since(requestStartTime)
						fmt.Printf("Bot %s - Total request time: %v\n", req.Device, totalDuration)
						fmt.Println("Bot " + speechReq.Device + " request served via Houndify.")
						return nil, nil
					}

					ttr.KnowledgeGraphResponseIG(req, apiResponse, transcribedText)
					totalDuration := time.Since(requestStartTime)
					fmt.Printf("Bot %s - Total request time: %v\n", req.Device, totalDuration)
					fmt.Println("Bot " + speechReq.Device + " request served via Houndify.")
					return nil, nil
				}
				// If Houndify fails or returns nothing useful, fall through to unmatched
				fmt.Println("Houndify returned empty or error response")
			} else {
				fmt.Println("Intent Graph: Text too short to be worth sending to intent graph")
			}
		}
		fmt.Println("No intent was matched.")
		ttr.IntentPass(req, "intent_system_unmatched", transcribedText, map[string]string{"": ""}, false)
		totalDuration := time.Since(requestStartTime)
		fmt.Printf("Bot %s - Total request time: %v\n", req.Device, totalDuration)
		return nil, nil
	}
	fmt.Println("Bot " + speechReq.Device + " request served.")
	totalDuration := time.Since(requestStartTime)
	fmt.Printf("Bot %s - Total request time: %v\n", req.Device, totalDuration)
	return nil, nil
}