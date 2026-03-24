package whisper

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	sr "cavalier/pkg/speechrequest"

	"cavalier/pkg/vars"

	whisper "github.com/kercre123/whisper.cpp/bindings/go"
)

var Name string = "whisper.cpp"

const numContexts = 2

type whisperContext struct {
	ctx    *whisper.Context
	params whisper.Params
}

var contextPool chan *whisperContext

func padPCM(data []byte) []byte {
	const sampleRate = 16000
	const minDurationMs = 1020
	const minDurationSamples = sampleRate * minDurationMs / 1000
	const bytesPerSample = 2

	currentSamples := len(data) / bytesPerSample

	if currentSamples >= minDurationSamples {
		return data
	}

	fmt.Println("Padding audio data to be 1000ms")

	paddingSamples := minDurationSamples - currentSamples
	paddingBytes := make([]byte, paddingSamples*bytesPerSample)

	return append(data, paddingBytes...)
}

func makeContext(modelPath string, sttLanguage string) (*whisperContext, error) {
	ctx := whisper.Whisper_init(modelPath)
	if ctx == nil {
		return nil, fmt.Errorf("failed to initialize whisper context")
	}
	params := ctx.Whisper_full_default_params(whisper.SamplingStrategy(whisper.SAMPLING_GREEDY))
	params.SetTranslate(false)
	params.SetPrintSpecial(false)
	params.SetPrintProgress(false)
	params.SetPrintRealtime(false)
	params.SetPrintTimestamps(false)
	params.SetThreads(runtime.NumCPU())
	params.SetNoContext(true)
	params.SetSingleSegment(true)
	params.SetLanguage(ctx.Whisper_lang_id(sttLanguage))
	return &whisperContext{ctx: ctx, params: params}, nil
}

func Init() error {
	whispModel := os.Getenv("WHISPER_MODEL")
	if whispModel == "" {
		fmt.Println("WHISPER_MODEL not defined, assuming tiny")
		whispModel = "tiny"
	} else {
		whispModel = strings.TrimSpace(whispModel)
	}
	var sttLanguage string
	if len(vars.APIConfig.STT.Language) == 0 {
		sttLanguage = "en"
	} else {
		sttLanguage = strings.Split(vars.APIConfig.STT.Language, "-")[0]
	}

	modelPath := filepath.Join("./whisper", "ggml.bin")
	if _, err := os.Stat(modelPath); err != nil {
		fmt.Println("Model does not exist: " + modelPath)
		return err
	}
	fmt.Println("Opening Whisper model (%s), creating %d contexts\n", modelPath, numContexts)

	contextPool = make(chan *whisperContext, numContexts)
	for i := 0; i < numContexts; i++ {
		wc, err := makeContext(modelPath, sttLanguage)
		if err != nil {
			return err
		}
		contextPool <- wc
		fmt.Printf("Created whisper context %d/%d\n", i+1, numContexts)
	}

	return nil
}

func STT(req sr.SpeechRequest) (string, error) {
	fmt.Println("(Bot " + req.Device + ", Whisper) Processing...")
	for {
		_, err := req.GetNextStreamChunk()
		if err != nil {
			return "", err
		}
		// has to be split into 320 []byte chunks for VAD
		speechIsDone, _ := req.DetectEndOfSpeech()
		if speechIsDone {
			break
		}
	}
	transcribedText, err := process(BytesToFloat32Buffer(padPCM(req.DecodedMicData)))
	if err != nil {
		return "", err
	}
	transcribedText = strings.ToLower(transcribedText)
	fmt.Println("Bot " + req.Device + " Transcribed text: " + transcribedText)
	return transcribedText, nil
}

func process(data []float32) (string, error) {
	wc := <-contextPool
	defer func() { contextPool <- wc }()

	var transcribedText string
	wc.ctx.Whisper_full(wc.params, data, nil, func(_ int) {
		transcribedText = strings.TrimSpace(wc.ctx.Whisper_full_get_segment_text(0))
	}, nil)
	return transcribedText, nil
}

func BytesToFloat32Buffer(buf []byte) []float32 {
	newB := make([]float32, len(buf)/2)
	factor := math.Pow(2, float64(16)-1)
	for i := 0; i < len(buf)/2; i++ {
		newB[i] = float32(float64(int16(binary.LittleEndian.Uint16(buf[i*2:]))) / factor)
	}
	return newB
}
